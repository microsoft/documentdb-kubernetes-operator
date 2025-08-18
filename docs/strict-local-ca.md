# Strict TLS with a local CA (cert-manager)

This guide shows an end-to-end path to run DocumentDB with strict TLS using a local CA issued by cert-manager. It uses the official GHCR images for DocumentDB and lets you mirror to your Azure Container Registry (ACR) if your policy requires it.

Assumptions
- You have an AKS cluster and kubectl context set.
- Azure Policy may restrict image registries. If GHCR is blocked, mirror required images to ACR or request a temporary exemption.
- cert-manager is installed and healthy. If not, install it as in the “TLS Happy Paths” doc.

What you’ll do
1) Prepare images so Azure Policy isn’t a blocker (choose one):
   - A. Create a temporary policy exemption for ghcr.io (CloudNativePG), or
   - B. Mirror the CloudNativePG image to your ACR and use that.
2) Install the DocumentDB operator via the local Helm chart using your ACR images.
3) Create a local CA (self-signed root → namespaced CA → Issuer) with cert-manager.
4) Create a DocumentDB resource that uses the Issuer (CertManager mode), then add a DNS name matching your Service.
5) Connect with strict TLS using the CA file from the issued secret.

Notes
- DocumentDB image: ghcr.io/microsoft/documentdb/documentdb-local:16 (or your mirrored ACR path).
- The gateway TLS secret issued by cert-manager will include ca.crt, tls.crt, tls.key.
- Use an SNI host that matches the certificate, e.g. <EXTERNAL-IP>.sslip.io.

---

## 0) Variables

```bash
export SUBSCRIPTION_ID="<your-subscription-guid>"
export RG="<your-resource-group>"
export AKS_NAME="<your-aks-name>"
export ACR_NAME="<youracr>"                 # e.g., guanzhoutest
export OPERATOR_IMAGE_REPO="$ACR_NAME.azurecr.io/documentdb/operator"
export SIDECAR_IMAGE_REPO="$ACR_NAME.azurecr.io/documentdb/sidecar"
export CNPG_IMAGE_REPO_ACR="$ACR_NAME.azurecr.io/cloudnative-pg"
export CNPG_IMAGE_TAG="1.27.0"              # known-good version
export IMAGE_TAG="$(date +%Y%m%d%H%M%S)"
```

---

## 1) Make CNPG image pass policy

Pick one route:

Option A — Policy exemption (fastest if you have rights)
- Create a policy exemption at the AKS scope for the “Allowed container registries” assignment.
- Use your org’s standard process. The exemption should allow ghcr.io for CloudNativePG pods in the operator namespace.

Option B — Mirror CNPG image to ACR (no policy changes)
- Import image into ACR, then use that repository in Helm values.

```bash
# Imports the public image into your ACR under $CNPG_IMAGE_REPO_ACR:$CNPG_IMAGE_TAG
az acr import -n "$ACR_NAME" \
  --source "ghcr.io/cloudnative-pg/cloudnative-pg:$CNPG_IMAGE_TAG" \
  --image "cloudnative-pg:$CNPG_IMAGE_TAG"
```

If import is blocked by ghcr.io, authenticate to GHCR or use a temporary policy exemption (Option A).

---

## 2) Build and push operator/sidecar to ACR

```bash
az acr login -n "$ACR_NAME"

docker build -t "$OPERATOR_IMAGE_REPO:$IMAGE_TAG" -f Dockerfile .
docker build -t "$SIDECAR_IMAGE_REPO:$IMAGE_TAG" -f plugins/sidecar-injector/Dockerfile plugins/sidecar-injector

docker push "$OPERATOR_IMAGE_REPO:$IMAGE_TAG"
docker push "$SIDECAR_IMAGE_REPO:$IMAGE_TAG"
```

---

## 3) Install/verify cert-manager

If not already installed, install cert-manager and verify pods are Running. See `docs/tls-happy-paths.md` for exact commands. Ensure the CRDs exist and the webhook is healthy.

---

## 4) Install the operator chart (override images)

```bash
kubectl create namespace documentdb-operator --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install documentdb-operator ./documentdb-chart \
  -n documentdb-operator \
  --wait --timeout 20m \
  --set image.documentdbk8soperator.repository="$OPERATOR_IMAGE_REPO" \
  --set image.documentdbk8soperator.tag="$IMAGE_TAG" \
  --set image.sidecarinjector.repository="$SIDECAR_IMAGE_REPO" \
  --set image.sidecarinjector.tag="$IMAGE_TAG" \
  --set cloudnative-pg.image.repository="$CNPG_IMAGE_REPO_ACR" \
  --set cloudnative-pg.image.tag="$CNPG_IMAGE_TAG"
```

Verify pods in namespace documentdb-operator are Running.

---

## 5) Create local CA (cert-manager)

Apply the example manifests in this repo to create a root, a namespaced CA, and an Issuer (adjust the namespace used for your workload; here we use documentdb-preview-ns):

```bash
# Create workload namespace
kubectl create namespace documentdb-preview-ns --dry-run=client -o yaml | kubectl apply -f -

# Create a root ClusterIssuer (self-signed), then a namespaced CA and Issuer
kubectl apply -f EXAMPLE_k8s_cert_management/ca-issuer/00-clusterissuer-selfsigned-root.yaml
kubectl -n documentdb-preview-ns apply -f EXAMPLE_k8s_cert_management/ca-issuer/01-certificate-root-ca.yaml
kubectl -n documentdb-preview-ns apply -f EXAMPLE_k8s_cert_management/ca-issuer/02-issuer-from-ca.yaml

# You should see secret documentdb-root-ca in documentdb-preview-ns
kubectl -n documentdb-preview-ns get secret documentdb-root-ca -o yaml | head -n 20
```

---

## 6) Credentials and DocumentDB resource (CertManager mode)

```bash
# App credentials used by the gateway
kubectl -n documentdb-preview-ns create secret generic documentdb-credentials \
  --from-literal=username="docdbuser" \
  --from-literal=password="P@ssw0rd123"

# Create the DocumentDB cluster referencing the namespaced Issuer
cat > /tmp/documentdb-certmanager.yaml <<'EOF'
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
    mode: CertManager
    certManager:
      issuerRef:
        name: documentdb-ca-issuer
        kind: Issuer
      # dnsNames will be added later once the Service gets an external IP
EOF

kubectl apply -f /tmp/documentdb-certmanager.yaml
```

Wait for pods and service:

```bash
kubectl -n documentdb-preview-ns get documentdb documentdb-preview -o yaml | sed -n '1,120p'
kubectl -n documentdb-preview-ns get pods -w
kubectl -n documentdb-preview-ns get svc -o wide
```

---

## 7) Add the external DNS name and reissue the cert

Get the external IP and set a matching SNI hostname using sslip.io, then patch the DocumentDB to include that DNS name so cert-manager reissues the leaf:

```bash
export SVC_IP=$(kubectl -n documentdb-preview-ns get svc documentdb-service-documentdb-preview -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export SNI_HOST="${SVC_IP}.sslip.io"

kubectl -n documentdb-preview-ns patch documentdb documentdb-preview --type merge \
  -p '{"spec":{"tls":{"certManager":{"dnsNames":["'"$SNI_HOST"'"]}}}}'

# Watch certificate become Ready and the gateway secret update
kubectl -n documentdb-preview-ns get documentdb documentdb-preview -o jsonpath='{.status.tls.secretName}{"\n"}'
kubectl -n documentdb-preview-ns get certificate -o wide
kubectl -n documentdb-preview-ns get secret $(kubectl -n documentdb-preview-ns get documentdb documentdb-preview -o jsonpath='{.status.tls.secretName}') -o json | jq '.data | keys'
```

You should see a secret name like documentdb-preview-gateway-cert that contains tls.crt, tls.key, and ca.crt.

---

## 8) Strict client test (mongosh)

Export the CA and connect using the SNI host without relaxed flags:

```bash
# Export CA file from the issued gateway secret
export GW_TLS_SECRET=$(kubectl -n documentdb-preview-ns get documentdb documentdb-preview -o jsonpath='{.status.tls.secretName}')
kubectl -n documentdb-preview-ns get secret "$GW_TLS_SECRET" -o jsonpath='{.data.ca\.crt}' | base64 -d > /tmp/ca.crt

# Fetch credentials
export USER=$(kubectl -n documentdb-preview-ns get secret documentdb-credentials -o jsonpath='{.data.username}' | base64 -d)
export PASS=$(kubectl -n documentdb-preview-ns get secret documentdb-credentials -o jsonpath='{.data.password}' | base64 -d)

# Connect (no tlsAllowInvalidCertificates) using the SNI host that matches dnsNames
mongosh "mongodb://$USER:$PASS@$SNI_HOST:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&replicaSet=rs0" \
  --tlsCAFile /tmp/ca.crt \
  --eval 'db.runCommand({ ping: 1 })'
```

Expected: `{ ok: 1 }` with a valid certificate chain and hostname match.

---

## 9) Troubleshooting

- Hostname mismatch
  - Ensure `.spec.tls.certManager.dnsNames` contains the SNI host you used (e.g., <IP>.sslip.io).
  - Reapply the DocumentDB resource to trigger re-issuance; confirm the Certificate is Ready.
- No ca.crt in secret
  - Verify the Issuer is backed by a CA (`02-issuer-from-ca.yaml`) and you are in CertManager mode.
- Image pull blocked by Azure Policy
  - Ensure operator and sidecar use your ACR images.
  - For CNPG, use Option A (policy exemption) or Option B (mirror to ACR) above.
- Service external IP is pending
  - Wait a few minutes; check your AKS LB provisioning and subnet.
- Sidecar args include certificate paths
  - The sidecar injector mounts the TLS secret and passes `--cert-path /tls/tls.crt` and `--key-file /tls/tls.key` automatically when the gateway TLS secret is present.

---

## 10) Cleanup (optional)

```bash
helm -n documentdb-operator uninstall documentdb-operator || true
kubectl delete ns documentdb-preview-ns || true
```

---

## Appendix: Where the CA manifests live

- EXAMPLE_k8s_cert_management/ca-issuer/00-clusterissuer-selfsigned-root.yaml
- EXAMPLE_k8s_cert_management/ca-issuer/01-certificate-root-ca.yaml
- EXAMPLE_k8s_cert_management/ca-issuer/02-issuer-from-ca.yaml

These create:
- A ClusterIssuer with a self-signed root
- A namespaced Certificate that stores `documentdb-root-ca` secret
- An Issuer that signs leaf certificates for the gateway
