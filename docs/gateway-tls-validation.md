# DocumentDB Gateway TLS End-to-End Validation

This guide combines the previous "TLS happy paths" and "Provided mode" documents into a single end-to-end walkthrough. It shows how to stand up the Azure infrastructure, deploy the operator, and validate the two supported gateway TLS modes:

- **SelfSigned** – operator provisions a namespaced self-signed issuer/certificate.
- **Provided** – gateway consumes a certificate sourced from Azure Key Vault via the Secrets Store CSI driver.

The flow is opinionated for a fresh environment. Adapt names or skip steps if resources already exist.

---

## Prerequisites
- Owner access to the target Azure subscription.
- CLI tools installed and on your `PATH`: `az`, `kubectl`, `helm`, `docker`, `mongosh`, `openssl`.
- Ability to authenticate with `az login`.

---

## 1. Environment Setup (AKS, ACR, cert-manager, CSI driver)

### 1.1 Set variables
```bash
export suffix="101301"
# export suffix=$(date +%m%d%H)
export SUBSCRIPTION_ID="81901d5e-31aa-46c5-b61a-537dbd5df1e7"
export LOCATION="eastus2"
export RG="documentdb-aks-${suffix}-rg"
export ACR_NAME="guanzhoutest"         # must be globally unique
export AKS_NAME="documentdb-aks-${suffix}"
export KV_NAME="ddb-issuer-${suffix}"
export NS="documentdb-preview-ns"
export DOCDB_NAME="documentdb-preview"
export DOCDB_VERSION="16"
export OPERATOR_IMAGE_REPO="$ACR_NAME.azurecr.io/documentdb/operator"
export SIDECAR_IMAGE_REPO="$ACR_NAME.azurecr.io/documentdb/sidecar"
export IMAGE_TAG="$(date +%Y%m%d%H%M%S)"
```

### 1.2 Select subscription
```bash
az account set --subscription "$SUBSCRIPTION_ID"
```

### 1.3 Create resource group
```bash
az group create -n "$RG" -l "$LOCATION"
```

### 1.4 Create Azure Container Registry
```bash
az acr create -g "$RG" -n "$ACR_NAME" --sku Standard
```

### 1.5 Create AKS with managed identity and attach ACR
```bash
az aks create -g "$RG" -n "$AKS_NAME" -l "$LOCATION" \
  --enable-managed-identity \
  --node-count 3 \
  -s Standard_d8s_v5 \
  --attach-acr "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RG/providers/Microsoft.ContainerRegistry/registries/$ACR_NAME"
```

### 1.6 Get kubeconfig credentials
```bash
az aks get-credentials -g "$RG" -n "$AKS_NAME" --overwrite-existing
```

### 1.7 Build and push operator images
```bash
az acr login -n "$ACR_NAME"
docker build -t "$OPERATOR_IMAGE_REPO:$IMAGE_TAG" -f Dockerfile .
docker build -t "$SIDECAR_IMAGE_REPO:$IMAGE_TAG" -f plugins/sidecar-injector/Dockerfile plugins/sidecar-injector
docker push "$OPERATOR_IMAGE_REPO:$IMAGE_TAG"
docker push "$SIDECAR_IMAGE_REPO:$IMAGE_TAG"
```

### 1.8 Cluster sanity checks
```bash
kubectl config current-context
kubectl cluster-info
kubectl get nodes -o wide
```

### 1.9 Install cert-manager
```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
kubectl create namespace cert-manager --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --set installCRDs=true
kubectl -n cert-manager get pods
```

If you hit connectivity errors, re-run the `az account set` and `az aks get-credentials` commands, then retry the helm install.

### 1.10 Install Secrets Store CSI driver + Azure provider
```bash
helm repo add csi-azure https://azure.github.io/secrets-store-csi-driver-provider-azure/charts
helm repo update

# Ensure the AKS add-on is not enabled concurrently. Disable it or skip Helm if it is.
kubectl -n kube-system get ds | grep -E 'aks-secrets-store-provider-azure' && echo "AKS add-on detected; disable it or skip Helm" || true
# az aks disable-addons -g "$RG" -n "$AKS_NAME" -a azure-keyvault-secrets-provider

helm uninstall csi-secrets-store -n kube-system || true
helm upgrade --install csi-azure-provider csi-azure/csi-secrets-store-provider-azure -n kube-system \
  --set "secrets-store-csi-driver.syncSecret.enabled=true"

kubectl -n kube-system wait --for=condition=Ready pod -l app=secrets-store-csi-driver --timeout=120s
kubectl -n kube-system wait --for=condition=Ready pod -l app=csi-secrets-store-provider-azure --timeout=120s
```

If you switch back to the managed add-on later, uninstall the Helm release first to avoid conflicts.

### 1.11 Deploy the operator Helm chart
```bash
kubectl create namespace documentdb-operator --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install documentdb-operator ./documentdb-chart \
  -n documentdb-operator \
  --set image.documentdbk8soperator.repository="$OPERATOR_IMAGE_REPO" \
  --set-string image.documentdbk8soperator.tag="$IMAGE_TAG" \
  --set image.sidecarinjector.repository="$SIDECAR_IMAGE_REPO" \
  --set-string image.sidecarinjector.tag="$IMAGE_TAG" \
  --set documentDbVersion="$DOCDB_VERSION"
kubectl -n documentdb-operator get pods
```

> Helm treats numeric values as numbers. Use `--set-string` for image tags to avoid scientific notation issues.

---

## 2. Prepare Workload Namespace + Credentials
```bash
kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create secret generic documentdb-credentials \
  --from-literal=username="docdbuser" \
  --from-literal=password="P@ssw0rd123" \
  --dry-run=client -o yaml | kubectl apply -f -
```

---

## 3. Validate SelfSigned TLS Mode

### 3.1 Create DocumentDB with SelfSigned TLS and LoadBalancer
```bash
cat > /tmp/documentdb-selfsigned.yaml <<'EOF'
apiVersion: db.microsoft.com/preview
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  nodeCount: 1
  instancesPerNode: 1
  documentDBVersion: "16"
  documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  gatewayImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  resource:
    pvcSize: 10Gi
  exposeViaService:
    serviceType: LoadBalancer
  tls:
    gateway:
      mode: SelfSigned
EOF

kubectl apply -f /tmp/documentdb-selfsigned.yaml
```

### 3.2 Monitor resource readiness
```bash
kubectl -n "$NS" get documentdb "$DOCDB_NAME" -o yaml | sed -n '1,140p'
kubectl -n "$NS" get pods -w
kubectl -n "$NS" get svc -o wide
```

Wait until the DocumentDB status shows `status.tls.ready: true` and note the generated secret name.

### 3.3 Capture LoadBalancer IP
```bash
export SVC_IP=$(kubectl -n "$NS" get svc documentdb-service-"$DOCDB_NAME" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export SNI_HOST="${SVC_IP}.sslip.io"
echo "SVC_IP=$SVC_IP"; echo "SNI_HOST=$SNI_HOST"
```

### 3.4 Connectivity check with relaxed TLS
```bash
export DOCDB_USER=$(kubectl -n "$NS" get secret documentdb-credentials -o jsonpath='{.data.username}' | base64 -d)
export DOCDB_PASS=$(kubectl -n "$NS" get secret documentdb-credentials -o jsonpath='{.data.password}' | base64 -d)

mongosh "mongodb://$DOCDB_USER:$DOCDB_PASS@$SVC_IP:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&tlsAllowInvalidCertificates=true&replicaSet=rs0" \
  --eval 'db.runCommand({ ping: 1 })'
```

You should see `{ ok: 1 }`. Because the certificate is self-signed, relaxed verification is expected unless you trust the self-signed CA.

---

## 4. Validate Provided TLS Mode (Azure Key Vault)

### 4.1 Ensure SelfSigned cluster is running
Keep the self-signed deployment from section 3; we will transition it to Provided mode so the LoadBalancer IP and gateway pods stay in place.

### 4.2 Authorize Azure Key Vault access
```bash
az keyvault create -g "$RG" -n "$KV_NAME" -l "$LOCATION" --enable-rbac-authorization true

MY_OBJECT_ID=$(az ad signed-in-user show --query id -o tsv)
az role assignment create --assignee-object-id "$MY_OBJECT_ID" \
  --role "Key Vault Certificates Officer" \
  --scope "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RG/providers/Microsoft.KeyVault/vaults/$KV_NAME"

KUBELET_MI_OBJECT_ID=$(az aks show -g "$RG" -n "$AKS_NAME" --query identityProfile.kubeletidentity.objectId -o tsv)
az role assignment create --assignee-object-id "$KUBELET_MI_OBJECT_ID" \
  --role "Key Vault Secrets User" \
  --scope "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RG/providers/Microsoft.KeyVault/vaults/$KV_NAME"
```

### 4.3 Create a server certificate for the gateway host
`$SNI_HOST` should still point at the LoadBalancer IP from section 3.

Option A – import an existing PFX:
```bash
export PFX_PATH=/path/to/cert_${SNI_HOST}.pfx
export PFX_PASSWORD="<pfx-password>"
az keyvault certificate import --vault-name "$KV_NAME" -n documentdb-gateway \
  --file "$PFX_PATH" --password "$PFX_PASSWORD"
```

Option B – create a self-signed cert in AKV with SAN set:
```bash
cat > /tmp/akv-cert-policy.json <<EOF
{
  "issuerParameters": { "name": "Self" },
  "x509CertificateProperties": {
    "subject": "CN=${SNI_HOST}",
    "subjectAlternativeNames": { "dnsNames": [ "${SNI_HOST}" ] },
    "keyUsage": [ "digitalSignature", "keyEncipherment" ],
    "validityInMonths": 12
  },
  "keyProperties": {
    "exportable": true,
    "keyType": "RSA",
    "keySize": 2048,
    "reuseKey": false
  },
  "secretProperties": { "contentType": "application/x-pem-file" }
}
EOF

az keyvault certificate create --vault-name "$KV_NAME" -n documentdb-gateway \
  --policy @/tmp/akv-cert-policy.json
```

### 4.4 Create SecretProviderClass with secret sync
```bash
env KV_NAME="$KV_NAME" kubectl apply -f - <<'EOF'
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: documentdb-azure-tls
  namespace: documentdb-preview-ns
spec:
  provider: azure
  secretObjects:
  - secretName: documentdb-provided-tls
    type: kubernetes.io/tls
    data:
    - objectName: "tls.crt"
      key: tls.crt
    - objectName: "tls.key"
      key: tls.key
  parameters:
    usePodIdentity: "false"
    useVMManagedIdentity: "true"
    keyvaultName: "${KV_NAME}"
    tenantId: "$(az account show --query tenantId -o tsv)"
    cloudName: "AzurePublicCloud"
    syncSecret: "true"
    objects: |
      array:
        - |
          objectName: "documentdb-gateway"
          objectType: "secret"
          objectAlias: "tls.crt"
          objectVersion: ""
        - |
          objectName: "documentdb-gateway"
          objectType: "secret"
          objectAlias: "tls.key"
          objectVersion: ""
EOF
```

If nodes use multiple user-assigned identities, set `userAssignedIdentityID` on the SecretProviderClass using the kubelet identity client ID and restart any helper pod.

### 4.5 Trigger secret sync with a helper deployment
```bash
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cert-puller
  namespace: documentdb-preview-ns
spec:
  replicas: 1
  selector:
    matchLabels: { app: cert-puller }
  template:
    metadata:
      labels: { app: cert-puller }
    spec:
      containers:
      - name: bb
        image: busybox
        command: ["sh","-c","sleep 3600"]
        volumeMounts:
        - name: secrets-store-inline
          mountPath: "/mnt/secrets-store"
          readOnly: true
      volumes:
      - name: secrets-store-inline
        csi:
          driver: secrets-store.csi.k8s.io
          readOnly: true
          volumeAttributes:
            secretProviderClass: "documentdb-azure-tls"
EOF
```

Check that the Kubernetes secret exists and has TLS keys:
```bash
kubectl -n "$NS" get secret documentdb-provided-tls -o jsonpath='{.type} {"\n"}'
kubectl -n "$NS" get secret documentdb-provided-tls -o jsonpath='{.data.tls\.crt}' | head -c 20; echo
kubectl -n "$NS" get secret documentdb-provided-tls -o jsonpath='{.data.tls\.key}' | head -c 20; echo
```

### 4.6 Switch DocumentDB to Provided mode
```bash
kubectl -n "$NS" patch documentdb "$DOCDB_NAME" --type merge -p "$(cat <<JSON
{
  "spec": {
    "tls": {
      "gateway": {
        "mode": "Provided",
        "provided": { "secretName": "documentdb-provided-tls" }
      }
    }
  }
}
JSON
)"
```

Watch status and CNPG plugin parameters:
```bash
kubectl -n "$NS" get documentdb "$DOCDB_NAME" -o yaml | sed -n '1,160p'
```

Expect to see:
- `status.tls.ready: true`
- `status.tls.secretName: documentdb-provided-tls`
- `status.tls.message: "Using provided TLS secret"`
- CNPG plugin parameters include `gatewayTLSSecret: documentdb-provided-tls`

### 4.7 Connect with mongosh (strict TLS)
```bash
export DOCDB_USER=$(kubectl -n "$NS" get secret documentdb-credentials -o jsonpath='{.data.username}' | base64 -d)
export DOCDB_PASS=$(kubectl -n "$NS" get secret documentdb-credentials -o jsonpath='{.data.password}' | base64 -d)

openssl s_client -connect "$SNI_HOST:10260" -servername "$SNI_HOST" -showcerts </dev/null \
  2>/dev/null | awk '/BEGIN CERTIFICATE/,/END CERTIFICATE/ {print}' > /tmp/ca.crt

mongosh "mongodb://$DOCDB_USER:$DOCDB_PASS@$SNI_HOST:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&replicaSet=rs0" \
  --tlsCAFile /tmp/ca.crt \
  --eval 'db.runCommand({ ping: 1 })'
```

If the SAN on the certificate does not match `$SNI_HOST`, add `&tlsAllowInvalidHostnames=true` temporarily while you rotate the certificate.

### 4.8 Tear down helper deployment (optional)
```bash
kubectl -n "$NS" delete deploy cert-puller || true
```

---

## 5. Troubleshooting Checklist
- **DocumentDB status** – `status.tls.message` will describe why TLS is not ready (missing secret keys, waiting for cert-manager, etc.).
- **Secrets Store CSI** – ensure the provider pods are running and the SecretProviderClass name matches the volume attribute on workloads.
- **Key Vault permissions** – kubelet identity needs "Key Vault Secrets User" on the vault; the operator/human account needs access to create/import certificates.
- **Certificate SAN** – must match the SNI host name you use when connecting (`<LB-IP>.sslip.io`, custom domain, etc.).
- **Image versions** – keep `documentDBVersion`, engine, and gateway images aligned to avoid pull failures.

---

## 6. Clean Up
```bash
kubectl -n "$NS" delete documentdb "$DOCDB_NAME" || true
kubectl -n "$NS" delete secret documentdb-provided-tls || true
kubectl -n "$NS" delete secret documentdb-credentials || true
kubectl -n "$NS" delete namespace "$NS" || true

helm uninstall documentdb-operator -n documentdb-operator || true
helm uninstall csi-azure-provider -n kube-system || true
helm uninstall cert-manager -n cert-manager || true

az group delete -n "$RG" --yes --no-wait
```

Adjust the cleanup commands if you want to retain shared infrastructure (for example, ACR or Key Vault).
