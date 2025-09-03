## TLS modes: happy-path validation summary

This document summarizes how we validated gateway TLS for the DocumentDB operator across the currently supported modes, and what to expect when it works.

Scope covered now:
- SelfSigned (cert-manager SelfSigned Issuer)
- Provided (secret managed outside the operator; Azure Key Vault + Secrets Store CSI example)
- CertManager (local CA Issuer via cert-manager)

Deferred (not included here):
- Azure Key Vault Issuer for cert-manager (requires GHCR-authenticated chart install and vault roles)

---

### Environment prerequisites
- AKS cluster with kubectl context set
- cert-manager installed (CRDs and controller running)
- Secrets Store CSI Driver + Azure provider installed (for Provided mode with AKV)
- The operator deployed, and a DocumentDB CR created (e.g., namespace `documentdb-preview-ns`, name `documentdb-preview`)

---

## 1) SelfSigned mode (quick smoke test)

Goal: Operator provisions a namespaced SelfSigned Issuer + Certificate; a TLS secret is produced and used by the gateway.

Steps (high level)
- Set the DocumentDB spec:
  - `spec.tls.mode: SelfSigned`
- Wait for status:
  - `status.tls.ready: true`
  - `status.tls.secretName: <generated secret name>`
- Operator syncs CNPG plugin param `gatewayTLSSecret` to match the secret.

Expected connection behavior
- Certificate is self-signed. Clients typically need relaxed verification (e.g., `--tlsAllowInvalidCertificates`) unless you add the self-signed cert to trust.

Success criteria
- DocumentDB status shows Ready with a TLS secret
- CNPG Cluster `.spec.plugins[].parameters.gatewayTLSSecret` set to the same secret
- TLS handshake succeeds (with relaxed verification)

---

## 2) Provided mode (AKV + SecretProviderClass)

Goal: Use a leaf cert that already exists outside the operator; sync it into Kubernetes as a TLS secret the operator can consume.

What we did
- Created an exportable server certificate in Azure Key Vault (CN/SAN = `<LB-IP>.sslip.io`), for example:
  - Vault: `ddb-issuer-01`
  - Certificate: `documentdb-gateway`
- Created a SecretProviderClass in `documentdb-preview-ns` to pull the AKV cert and `syncSecret: true` to produce:
  - Secret: `documentdb-provided-tls` (contains `tls.crt` and `tls.key`)
- Switched the DocumentDB CR to:
  - `spec.tls.mode: Provided`
  - `spec.tls.provided.secretName: documentdb-provided-tls`

Observed results (from our run)
- `status.tls.ready: true`
- `status.tls.secretName: documentdb-provided-tls`
- `status.tls.message: "Using provided TLS secret"`
- CNPG plugin parameters show:
  - `gatewayTLSSecret: documentdb-provided-tls`
- Connectivity test (mongosh):
  - Host: `<LB-IP>.sslip.io` (example: `4.152.46.142.sslip.io`)
  - Command with relaxed TLS: `{ ok: 1 }`

Notes
- The example AKV certificate used a self-signed leaf (AKV Self policy). For strict verification, clients must trust the signer (use a CA-backed chain or a trusted CA file). See next section for a local CA.

Success criteria
- The synced secret exists and contains tls.crt/tls.key
- DocumentDB shows Ready using that secret; CNPG plugin param updated
- TLS handshake works (relaxed flags for a self-signed leaf, or strict if you supply a trusted chain)

---

## 3) CertManager mode with a local CA Issuer (strict TLS)

Goal: Create a local CA via cert-manager, then issue the gateway leaf from that CA so clients can validate strictly using the included `ca.crt`.

Manifests (in repo)
- `EXAMPLE_k8s_cert_management/ca-issuer/00-clusterissuer-selfsigned-root.yaml`
- `EXAMPLE_k8s_cert_management/ca-issuer/01-certificate-root-ca.yaml` (namespaced CA, secret `documentdb-root-ca`)
- `EXAMPLE_k8s_cert_management/ca-issuer/02-issuer-from-ca.yaml` (namespaced Issuer `documentdb-ca-issuer`)

Steps (high level)
1. Apply the three manifests above.
2. Patch DocumentDB to:
   - `spec.tls.mode: CertManager`
   - `spec.tls.certManager.issuerRef: { name: documentdb-ca-issuer, kind: Issuer }`
   - Optionally include external SNI host (e.g., `<LB-IP>.sslip.io`) in `spec.tls.certManager.dnsNames`.
3. Wait for certificate Ready. cert-manager will include `ca.crt` in the issued secret.
4. Connect with strict TLS by supplying the CA file (e.g., `--tlsCAFile /tmp/ca.crt`).

Expected outcomes
- DocumentDB status shows Ready with cert-manager-managed secret
- Secret contains: `tls.crt`, `tls.key`, and `ca.crt`
- Strict TLS validation succeeds when clients use `ca.crt`

---

## Not included here: AKV Issuer for cert-manager

Reason
- Installing the Azure Key Vault Issuer controller currently requires authenticated access to GHCR (Helm OCI) and appropriate Key Vault data-plane roles. This was out of scope during this run.

Next steps if needed
- Authenticate to GHCR and install the AKV Issuer controller
- Create an Issuer referencing your AKV CA
- Switch DocumentDB to `CertManager` mode with that issuer
- Clients validate strictly using the CA chain

---

## Quick verification checklist (applies to all modes)
- DocumentDB:
  - `status.tls.ready: true`
  - `status.tls.secretName` matches the in-use TLS secret
  - `status.tls.message` is informative (e.g., Provided/SelfSigned/CertManager ready)
- CNPG Cluster:
  - `.spec.plugins[].parameters.gatewayTLSSecret` equals the secret above
- Connectivity:
  - Use SNI host that matches the certificate (e.g., `<LB-IP>.sslip.io`)
  - For strict validation, provide a trusted CA (`ca.crt`); otherwise use relaxed flags for self-signed leaves

---

## End-to-end guide (from zero): SelfSigned mode

This section provides one-command-at-a-time steps to create Azure resources, build and push images to ACR, deploy the operator via Helm, create a sample cluster with SelfSigned TLS, expose it, and connect using `mongosh`.

Assumptions
- You have Owner on the subscription.
- You’re running on Linux with bash.
- You can authenticate with `az login` and have permissions to create resources.

Set variables
```
export SUBSCRIPTION_ID="81901d5e-31aa-46c5-b61a-537dbd5df1e7"
export LOCATION="eastus2"
export RG="documentdb-aks4-rg"
export ACR_NAME="guanzhoutest"           # use this ACR name as provided
export AKS_NAME="documentdb-aks4"
export KV_NAME="ddb-issuer-04"           # optional for AKV prep (unique in your tenant)
export OPERATOR_IMAGE_REPO="$ACR_NAME.azurecr.io/documentdb/operator"
export SIDECAR_IMAGE_REPO="$ACR_NAME.azurecr.io/documentdb/sidecar"
export IMAGE_TAG="$(date +%Y%m%d%H%M%S)"

echo "Using variables:"
echo "SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
echo "LOCATION=$LOCATION"
echo "RG=$RG"
echo "ACR_NAME=$ACR_NAME"
echo "AKS_NAME=$AKS_NAME"
echo "KV_NAME=$KV_NAME"
echo "OPERATOR_IMAGE_REPO=$OPERATOR_IMAGE_REPO"
echo "SIDECAR_IMAGE_REPO=$SIDECAR_IMAGE_REPO"
echo "IMAGE_TAG=$IMAGE_TAG"
```

1) Select subscription
```
az account set --subscription "$SUBSCRIPTION_ID"
```

2) Create resource group
```
az group create -n "$RG" -l "$LOCATION"
```

3) Create ACR (if not already present)
```
az acr create -g "$RG" -n "$ACR_NAME" --sku Standard_d8s_v5
```

4) Create AKS with managed identity and attach ACR
```
az aks create -g "$RG" -n "$AKS_NAME" -l "$LOCATION" \
  --enable-managed-identity \
  --node-count 3 \
  -s Standard_d8s_v5 \
  --attach-acr "/subscriptions/81901d5e-31aa-46c5-b61a-537dbd5df1e7/resourceGroups/guanzhou-test-rg/providers/Microsoft.ContainerRegistry/registries/guanzhoutest"
```

5) Get kubeconfig credentials
```
az aks get-credentials -g "$RG" -n "$AKS_NAME" --overwrite-existing
```

Optional: Prepare Azure Key Vault (for Provided mode later)
- Create a Key Vault and grant the AKS kubelet identity data-plane access to read secrets.
```
az keyvault create -g "$RG" -n "$KV_NAME" -l "$LOCATION" --enable-rbac-authorization true
```
```
KUBELET_MI_OBJECT_ID=$(az aks show -g "$RG" -n "$AKS_NAME" --query identityProfile.kubeletidentity.objectId -o tsv)
```
```
az role assignment create  --assignee-object-id "$KUBELET_MI_OBJECT_ID" \
  --role "Key Vault Secrets User" \
  --scope "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RG/providers/Microsoft.KeyVault/vaults/$KV_NAME"
```

6) Login to ACR
```
az acr login -n "$ACR_NAME"
```

7) Build and push images (operator and sidecar injector)
```
docker build -t "$OPERATOR_IMAGE_REPO:$IMAGE_TAG" -f Dockerfile . 
```
```
docker build -t "$SIDECAR_IMAGE_REPO:$IMAGE_TAG" -f plugins/sidecar-injector/Dockerfile plugins/sidecar-injector
```
```
docker push "$OPERATOR_IMAGE_REPO:$IMAGE_TAG"
```
```
docker push "$SIDECAR_IMAGE_REPO:$IMAGE_TAG"
```

8) Install cert-manager (required for Certificate resources)
```
helm repo add jetstack https://charts.jetstack.io

helm repo update

kubectl create namespace cert-manager

helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --set installCRDs=true

kubectl -n cert-manager get pods
```

9) Deploy the operator Helm chart using your ACR images
```
kubectl create namespace documentdb-operator
```
```
helm upgrade --install documentdb-operator ./documentdb-chart \
  -n documentdb-operator \
  --set image.documentdbk8soperator.repository="$OPERATOR_IMAGE_REPO" \
  --set image.documentdbk8soperator.tag="$IMAGE_TAG" \
  --set image.sidecarinjector.repository="$SIDECAR_IMAGE_REPO" \
  --set image.sidecarinjector.tag="$IMAGE_TAG"
```
```
kubectl -n documentdb-operator get pods
```

10) Create a workload namespace and credentials secret
```
kubectl create namespace documentdb-preview-ns
```
```
kubectl -n documentdb-preview-ns create secret generic documentdb-credentials \
  --from-literal=username="docdbuser" \
  --from-literal=password="P@ssw0rd123"
```

11) Create a DocumentDB cluster with SelfSigned TLS and LoadBalancer
Option A: Patch the sample file and apply.
```
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
```
```
kubectl apply -f /tmp/documentdb-selfsigned.yaml
```

12) Verify operator, CNPG, and cluster readiness
```
kubectl -n documentdb-preview-ns get documentdb documentdb-preview -o yaml | sed -n '1,120p'
```
```
kubectl -n documentdb-preview-ns get pods -w
```
```
kubectl -n documentdb-preview-ns get svc -o wide
```

13) Get external IP and test connectivity with mongosh
```
export SVC_IP=$(kubectl -n documentdb-preview-ns get svc documentdb-service-documentdb-preview -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo $SVC_IP
```
```
export USER=$(kubectl -n documentdb-preview-ns get secret documentdb-credentials -o jsonpath='{.data.username}' | base64 -d)

echo $USER
```
```
export PASS=$(kubectl -n documentdb-preview-ns get secret documentdb-credentials -o jsonpath='{.data.password}' | base64 -d)

echo $PASS
```
mongosh "mongodb://$USER:$PASS@$SVC_IP:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&tlsAllowInvalidCertificates=true&replicaSet=rs0" --eval 'db.runCommand({ ping: 1 })'
```

Expected result
- The `ping` command returns `{ ok: 1 }`.
- The TLS handshake succeeds with `tlsAllowInvalidCertificates=true` because the self-signed leaf is not trusted by default.

Troubleshooting quick checks
- Ensure cert-manager pods are Running.
- Ensure operator pods in `documentdb-operator` are Running.
- Check DocumentDB status conditions for TLS readiness and secret name.
- If `EXTERNAL-IP` is `<pending>`, wait a few minutes and re-check the Service.
