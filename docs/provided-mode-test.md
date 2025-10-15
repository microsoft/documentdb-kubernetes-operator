# Provided TLS mode: step-by-step test (AKV + Secrets Store CSI)

This guide shows how to validate Provided TLS mode end-to-end using Azure Key Vault and the Secrets Store CSI Driver with managed identity. It complements `docs/tls-happy-paths.md` section “Provided mode”.

## What you’ll do
- Create or reuse a DocumentDB cluster exposed via LoadBalancer
- Mint a server certificate in Azure Key Vault for the service’s SNI host (<LB-IP>.sslip.io)
- Use a SecretProviderClass to sync the AKV cert into a Kubernetes TLS secret
- Switch DocumentDB to `spec.tls.mode: Provided` and point it at that secret
- Connect with mongosh

## Prerequisites
- You are Owner on the target Azure subscription
- Tools on your machine: Azure CLI, Docker, kubectl, and Helm
  - Login later with `az login`
  - We’ll create all Azure resources (RG, AKS, Key Vault) and install all cluster add-ons (cert-manager, CSI + Azure provider). You’ll also push custom images to GHCR.

Repo examples you can reference:
- `EXAMPLE_k8s_cert_management/azure-key-vault/azure-secret-provider-class.yaml`
- `EXAMPLE_k8s_cert_management/azure-key-vault/busybox-cert-puller.yaml`

## Set variables
```bash
export suffix=$(date +%m%d%H)
export SUBSCRIPTION_ID="81901d5e-31aa-46c5-b61a-537dbd5df1e7"
export LOCATION="eastus2"
export RG="documentdb-aks-${suffix}-rg"
export AKS_NAME="documentdb-aks-${suffix}"
export KV_NAME="${USER}-AKV-${suffix}"
export NS="documentdb-preview-ns"
export DOCDB_NAME="documentdb-preview"
export CERT_NAME="documentdb-gateway"
export SECRET_NAME="documentdb-provided-tls"
export GHCR_USER="guanzhousongmicrosoft"                             # GitHub username with push access
export GHCR_PAT="<ghcr-personal-access-token-write-packages>" # scoped for write/read packages
export OPERATOR_IMAGE_REPO="ghcr.io/guanzhousongmicrosoft/documentdb-kubernetes-operator/operator"
export SIDECAR_IMAGE_REPO="ghcr.io/guanzhousongmicrosoft/documentdb-kubernetes-operator/sidecar"
export IMAGE_TAG="$(date +%Y%m%d)"

echo "suffix=$suffix"
echo "SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
echo "LOCATION=$LOCATION"
echo "RG=$RG"
echo "AKS_NAME=$AKS_NAME"
echo "KV_NAME=$KV_NAME"
echo "NS=$NS"
echo "DOCDB_NAME=$DOCDB_NAME"
echo "CERT_NAME=$CERT_NAME"
echo "SECRET_NAME=$SECRET_NAME"
echo "GHCR_USER=$GHCR_USER"
echo "OPERATOR_IMAGE_REPO=$OPERATOR_IMAGE_REPO"
echo "SIDECAR_IMAGE_REPO=$SIDECAR_IMAGE_REPO"
echo "IMAGE_TAG=$IMAGE_TAG"
```

Select subscription
```bash
az account set --subscription "$SUBSCRIPTION_ID"
```

## 0) Azure + AKS setup (from zero)

Create resource group:
```bash
az group create -n "$RG" -l "$LOCATION"
```


Create AKS with managed identity:
```bash
az aks create -g "$RG" -n "$AKS_NAME" -l "$LOCATION" \
  --enable-managed-identity \
  --node-count 3 \
  -s Standard_d8s_v5 \
  --generate-ssh-keys
```

Get kubeconfig credentials:
```bash
az aks get-credentials -g "$RG" -n "$AKS_NAME" --overwrite-existing
```

build images in Github...


Preflight: verify cluster connectivity (fix before proceeding):
```bash
kubectl config current-context
kubectl cluster-info
kubectl get nodes -o wide
```

Install cert-manager (CRDs + controller):
```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
kubectl create namespace cert-manager --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --set installCRDs=true
kubectl -n cert-manager get pods
```
If you see errors like “Kubernetes cluster unreachable” or “no such host” during these steps, reselect your subscription and reacquire AKS credentials, then retry:
```bash
az account set --subscription "$SUBSCRIPTION_ID"
az aks list -o table
az aks get-credentials -g "$RG" -n "$AKS_NAME" --overwrite-existing
kubectl cluster-info && kubectl get nodes
```

Install Azure provider (bundled driver; enable secret sync):
```bash
helm repo add csi-azure https://azure.github.io/secrets-store-csi-driver-provider-azure/charts
helm repo update
# IMPORTANT: Do not mix with the AKS managed add-on. If it's enabled, disable it first or skip Helm.
kubectl -n kube-system get ds | grep -E 'aks-secrets-store-provider-azure' && echo "AKS add-on detected; disable it or skip Helm" || true


# Install the Azure provider; it bundles the CSI driver. Enable secret sync on the bundled driver.
helm upgrade --install csi-azure-provider csi-azure/csi-secrets-store-provider-azure -n kube-system \
  --set "secrets-store-csi-driver.syncSecret.enabled=true"
kubectl -n kube-system get pods -l app=secrets-store-csi-driver
kubectl -n kube-system get pods -l app=csi-secrets-store-provider-azure

# Verify readiness and DaemonSets exist
kubectl -n kube-system wait --for=condition=Ready pod -l app=secrets-store-csi-driver --timeout=120s
kubectl -n kube-system wait --for=condition=Ready pod -l app=csi-secrets-store-provider-azure --timeout=120s
kubectl -n kube-system get ds -l app=secrets-store-csi-driver -o wide
kubectl -n kube-system get ds -l app=csi-secrets-store-provider-azure -o wide

> Important: The SecretProviderClass `spec.provider` must be `azure` (lowercase), and any pod mounting it must set `volumeAttributes.secretProviderClass` to the same name.

Deploy the operator Helm chart using your GHCR images:
```bash
kubectl create namespace documentdb-operator --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install documentdb-operator ./documentdb-chart \
  -n documentdb-operator \
  --set image.documentdbk8soperator.repository="$OPERATOR_IMAGE_REPO" \
  --set image.documentdbk8soperator.tag="$IMAGE_TAG" \
  --set image.sidecarinjector.repository="$SIDECAR_IMAGE_REPO" \
  --set image.sidecarinjector.tag="$IMAGE_TAG"
kubectl -n documentdb-operator get pods
```

If your GHCR repositories are private, create a `docker-registry` secret in `documentdb-operator` and `documentdb-preview-ns`, then set `imagePullSecrets` in the chart values or via `--set imagePullSecrets[0].name=...`.

## 1) Ensure a DocumentDB Service with an external IP
If you already have a cluster with a LoadBalancer service, skip to step 2.

Create namespace and credentials (if needed):
```bash
kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create secret generic documentdb-credentials \
  --from-literal=username="docdbuser" \
  --from-literal=password="P@ssw0rd123" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Create a temporary SelfSigned cluster to get a LoadBalancer IP (we’ll switch to Provided later):
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
  documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  resource:
    pvcSize: 10Gi
  exposeViaService:
    serviceType: LoadBalancer
  tls:
    mode: SelfSigned
EOF
kubectl apply -f /tmp/documentdb-selfsigned.yaml
```

Wait for the service and capture the IP:
```bash
kubectl -n "$NS" get svc -w
```
```bash
export SVC_IP=$(kubectl -n "$NS" get svc documentdb-service-"$DOCDB_NAME" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export SNI_HOST="${SVC_IP}.sslip.io"
echo "SVC_IP=$SVC_IP"; echo "SNI_HOST=$SNI_HOST"
```

## 2) Prepare Azure Key Vault and grant AKS kubelet access
Create Key Vault (if not already present). Grant your human account cert permissions to import/create, and grant the cluster’s kubelet identity read access to secrets:
```bash
az keyvault create -g "$RG" -n "$KV_NAME" -l "$LOCATION" --enable-rbac-authorization true
```

1. Add your account to have Role assigned: Key Vault Certificates Officer
2. Add your AKS cluster managed identity to have role assigned: Key Vault Secrets User




## 3) Create a server certificate in AKV for the SNI host
Create (or import) a certificate whose CN/SAN matches `$SNI_HOST`. The private key must be exportable.

Option A: Import a PFX you have already prepared for `$SNI_HOST`:
```bash
# PFX must include the private key; set the password env var if needed
export PFX_PATH=/path/to/cert_${SNI_HOST}.pfx
export PFX_PASSWORD="<pfx-password>"
az keyvault certificate import --vault-name "$KV_NAME" -n "$CERT_NAME" \
  --file "$PFX_PATH" --password "$PFX_PASSWORD"
```
Important: The certificate’s SAN must include `$SNI_HOST` (e.g., `<LB-IP>.sslip.io`). If it doesn’t, strict hostname verification will fail.

Option B: Create a self-signed certificate in AKV (quick test):
```bash
# Uses the default policy as a base; most tenants will need a custom policy
# for exportable keys. If default isn’t exportable, prefer Option A.
az keyvault certificate create \
  --vault-name "$KV_NAME" -n "$CERT_NAME" \
  --policy "$(az keyvault certificate get-default-policy)"
```
Note: For strict client verification, use a CA-backed certificate or a chain your client trusts.

## 4) Create a SecretProviderClass to sync the TLS secret
We’ll sync a Kubernetes TLS secret named `$SECRET_NAME` in namespace `$NS` that contains `tls.crt` and `tls.key`.

Create the SecretProviderClass manifest with a heredoc:
```bash
cat > /tmp/azure-secret-provider-class.yaml <<'EOF'
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
    - objectName: "tls.crt"   # must match a file name created by the provider
      key: tls.crt
    - objectName: "tls.key"   # must match a file name created by the provider
      key: tls.key
  parameters:
    usePodIdentity: "false"
    useVMManagedIdentity: "true"
    # userAssignedIdentityID: "<client-id>"   # set if your nodes have multiple user-assigned identities
    keyvaultName: "${KV_NAME}"
    tenantId: "$(az account show --query tenantId -o tsv)"
    cloudName: "AzurePublicCloud"
    syncSecret: "true"
    objects: |
      array:
        - |
          objectName: "${CERT_NAME}"
          objectType: "secret"
          objectAlias: "tls.crt"   # provider will emit this file; referenced above
          objectVersion: ""
        - |
          objectName: "${CERT_NAME}"
          objectType: "secret"
          objectAlias: "tls.key"   # provider will emit this file; referenced above
          objectVersion: ""
EOF

env CERT_NAME="$CERT_NAME" KV_NAME="$KV_NAME" envsubst < /tmp/azure-secret-provider-class.yaml | kubectl apply -f -

# If you see IMDS errors like "Multiple user assigned identities exist" in pod events,
# set the kubelet user-assigned identity clientId explicitly on the SPC and restart the puller:
KUBELET_CLIENT_ID=$(az aks show -g "$RG" -n "$AKS_NAME" --query identityProfile.kubeletidentity.clientId -o tsv)
kubectl -n "$NS" patch secretproviderclass documentdb-azure-tls --type merge -p '{"spec":{"parameters":{"userAssignedIdentityID":"'"$KUBELET_CLIENT_ID"'"}}}'
kubectl -n "$NS" rollout restart deploy cert-puller || true
```

Trigger the sync using a tiny “cert puller” pod that mounts the CSI volume:
```bash
cat > /tmp/busybox-cert-puller.yaml <<'EOF'
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
            secretProviderClass: "documentdb-azure-tls"   # must match SecretProviderClass.metadata.name
EOF
kubectl apply -f /tmp/busybox-cert-puller.yaml
```

Wait for the synced secret and verify keys:
```bash
kubectl -n "$NS" get secret "$SECRET_NAME" -o jsonpath='{.type}{"\n"}'
kubectl -n "$NS" get secret "$SECRET_NAME" -o jsonpath='{.data.tls\.crt}{"\n"}' | head -c 20; echo
kubectl -n "$NS" get secret "$SECRET_NAME" -o jsonpath='{.data.tls\.key}{"\n"}' | head -c 20; echo
```
Type must be `kubernetes.io/tls` and both keys should exist.


## 5) Switch DocumentDB to Provided TLS
Patch the CR to use the synced secret:
```bash
kubectl -n "$NS" patch documentdb "$DOCDB_NAME" --type merge -p "$(cat <<JSON
{
  "spec": {
    "tls": {
      "mode": "Provided",
      "provided": { "secretName": "$SECRET_NAME" }
    }
  }
}
JSON
)"
```

Confirm status and CNPG plugin parameter:
```bash
kubectl -n "$NS" get documentdb "$DOCDB_NAME" -o yaml | sed -n '1,140p'
```
Look for:
- `status.tls.ready: true`
- `status.tls.secretName: $SECRET_NAME`
- CNPG plugin parameters include `gatewayTLSSecret: $SECRET_NAME`

## 6) Connect with mongosh
Get credentials and connect using the SNI host that matches the cert:
```bash
export DOCDB_USER=$(kubectl -n "$NS" get secret documentdb-credentials -o jsonpath='{.data.username}' | base64 -d)
export DOCDB_PASS=$(kubectl -n "$NS" get secret documentdb-credentials -o jsonpath='{.data.password}' | base64 -d)
```

Build a CA file from the presented chain (handy for self-signed or ad-hoc tests):
```bash
openssl s_client -connect "$SNI_HOST:10260" -servername "$SNI_HOST" -showcerts </dev/null \
  2>/dev/null | awk '/BEGIN CERTIFICATE/,/END CERTIFICATE/ {print}' > /tmp/ca.crt
```

TODO: WHY NOT WORK? ASK
Strict TLS (requires SAN match and trust):
```bash
mongosh "mongodb://$SNI_HOST:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&replicaSet=rs0" \
  --tlsCAFile /tmp/ca.crt \
  -u "$DOCDB_USER" -p "$DOCDB_PASS" \
  --eval 'db.runCommand({ ping: 1 })'
```

Relaxed hostname (keeps CA trust but bypasses SAN mismatch):
```bash
mongosh "mongodb://$SNI_HOST:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&replicaSet=rs0&tlsAllowInvalidHostnames=true" \
  --tlsCAFile /tmp/ca.crt \
  -u "$DOCDB_USER" -p "$DOCDB_PASS" \
  --eval 'db.runCommand({ ping: 1 })'
```
Expected: `{ ok: 1 }`.

Strict TLS (only if clients trust the signer):
- Provide a CA file clients trust, or use a CA-backed certificate in AKV and include the chain.

## Troubleshooting
- Secret not created: ensure the puller pod is running and `secretProviderClass` name matches; check `kubectl describe spc documentdb-azure-tls` and pod events.
- Provider not found: if you see `provider not found: provider "azure"`, ensure the Azure provider is installed and running
  - `kubectl -n kube-system get pods -l app=csi-secrets-store-provider-azure`
  - `spec.provider` in SecretProviderClass is `azure` (exact case), and the puller pod’s `volumeAttributes.secretProviderClass` matches the SPC name.
- AKV permission: verify kubelet managed identity has Key Vault data-plane role “Key Vault Secrets User” on the vault; your human account needs “Certificates Officer/Admin” to import/create.
- DocumentDB status not Ready: confirm `status.tls.message` and that secret type is `kubernetes.io/tls` with `tls.crt` and `tls.key`.
- Certificate SAN mismatch: ensure `$SNI_HOST` matches your cert (e.g., `<LB-IP>.sslip.io`). If you can’t rotate the cert yet, add `&tlsAllowInvalidHostnames=true` temporarily.

## Clean up
```bash
kubectl -n "$NS" delete deploy cert-puller || true

  Fetch the chart dependency required by the operator (CloudNativePG):
  ```bash
  helm dependency update ./documentdb-chart
  ```

kubectl -n "$NS" delete secret "$SECRET_NAME" || true
kubectl -n "$NS" delete secret documentdb-credentials || true
kubectl -n "$NS" delete documentdb "$DOCDB_NAME" || true
```

---

Notes
- For rotation, set `rotationPollInterval` in the SecretProviderClass and rely on CSI sync. Restart pods that load the secret if hot reload is not supported.
- If you use a user-assigned managed identity, set `userAssignedIdentityID` and ensure it has Key Vault access.
