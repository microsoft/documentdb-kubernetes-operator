# Defaults
# renovate: datasource=github-releases depName=kubernetes-csi/csi-driver-host-path
CSI_DRIVER_HOST_PATH_DEFAULT_VERSION=v1.17.0
# renovate: datasource=github-releases depName=kubernetes-csi/external-snapshotter
EXTERNAL_SNAPSHOTTER_VERSION=v8.4.0
# renovate: datasource=github-releases depName=kubernetes-csi/external-provisioner
EXTERNAL_PROVISIONER_VERSION=v6.0.0
# renovate: datasource=github-releases depName=kubernetes-csi/external-resizer
EXTERNAL_RESIZER_VERSION=v2.0.0
# renovate: datasource=github-releases depName=kubernetes-csi/external-attacher
EXTERNAL_ATTACHER_VERSION=v4.10.0

CSI_DRIVER_HOST_PATH_VERSION=${CSI_DRIVER_HOST_PATH_VERSION:-$CSI_DRIVER_HOST_PATH_DEFAULT_VERSION}

TEMP_DIR="$(mktemp -d)"

# Colors (only if using a terminal)
bright=
reset=
if [ -t 1 ]; then
  bright=$(tput bold 2>/dev/null || true)
  reset=$(tput sgr0 2>/dev/null || true)
fi

echo "${bright}Starting deployment of CSI driver plugin... ${reset}"
CSI_BASE_URL=https://raw.githubusercontent.com/kubernetes-csi

## Install external snapshotter CRD
kubectl apply -f "${CSI_BASE_URL}"/external-snapshotter/"${EXTERNAL_SNAPSHOTTER_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl apply -f "${CSI_BASE_URL}"/external-snapshotter/"${EXTERNAL_SNAPSHOTTER_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
kubectl apply -f "${CSI_BASE_URL}"/external-snapshotter/"${EXTERNAL_SNAPSHOTTER_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
kubectl apply -f "${CSI_BASE_URL}"/external-snapshotter/"${EXTERNAL_SNAPSHOTTER_VERSION}"/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
kubectl apply -f "${CSI_BASE_URL}"/external-snapshotter/"${EXTERNAL_SNAPSHOTTER_VERSION}"/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
kubectl apply -f "${CSI_BASE_URL}"/external-snapshotter/"${EXTERNAL_SNAPSHOTTER_VERSION}"/deploy/kubernetes/csi-snapshotter/rbac-csi-snapshotter.yaml

## Install external provisioner
kubectl apply -f "${CSI_BASE_URL}"/external-provisioner/"${EXTERNAL_PROVISIONER_VERSION}"/deploy/kubernetes/rbac.yaml

## Install external attacher
kubectl apply -f "${CSI_BASE_URL}"/external-attacher/"${EXTERNAL_ATTACHER_VERSION}"/deploy/kubernetes/rbac.yaml

## Install external resizer
kubectl apply -f "${CSI_BASE_URL}"/external-resizer/"${EXTERNAL_RESIZER_VERSION}"/deploy/kubernetes/rbac.yaml

## Install driver and plugin
## Create a temporary file for the modified plugin deployment. This is needed
## because csi-driver-host-path plugin yaml tends to lag behind a few versions.
plugin_file="${TEMP_DIR}/csi-hostpath-plugin.yaml"
curl -sSL "${CSI_BASE_URL}/csi-driver-host-path/${CSI_DRIVER_HOST_PATH_VERSION}/deploy/kubernetes-1.30/hostpath/csi-hostpath-plugin.yaml" |
  sed "s|registry.k8s.io/sig-storage/hostpathplugin:.*|registry.k8s.io/sig-storage/hostpathplugin:${CSI_DRIVER_HOST_PATH_VERSION}|g" > "${plugin_file}"

kubectl apply -f "${CSI_BASE_URL}"/csi-driver-host-path/"${CSI_DRIVER_HOST_PATH_VERSION}"/deploy/kubernetes-1.30/hostpath/csi-hostpath-driverinfo.yaml
kubectl apply -f "${plugin_file}"
rm "${plugin_file}"

## create volumesnapshotclass
kubectl apply -f "${CSI_BASE_URL}"/csi-driver-host-path/"${CSI_DRIVER_HOST_PATH_VERSION}"/deploy/kubernetes-1.30/hostpath/csi-hostpath-snapshotclass.yaml
kubectl patch volumesnapshotclass csi-hostpath-snapclass -p '{"metadata":{"annotations":{"snapshot.storage.kubernetes.io/is-default-class":"true"}}}' --type merge

## Prevent VolumeSnapshot E2e test to fail when taking a
## snapshot of a running PostgreSQL instance
kubectl patch volumesnapshotclass csi-hostpath-snapclass -p '{"parameters":{"ignoreFailedRead":"true"}}' --type merge

## create storage class
kubectl apply -f "${CSI_BASE_URL}"/csi-driver-host-path/"${CSI_DRIVER_HOST_PATH_VERSION}"/examples/csi-storageclass.yaml
kubectl annotate storageclass csi-hostpath-sc storage.kubernetes.io/default-snapshot-class=csi-hostpath-snapclass

echo "${bright} CSI driver plugin deployment has started. Waiting for the CSI plugin to be ready... ${reset}"
ITER=0
while true; do
  if [[ $ITER -ge 300 ]]; then
    echo "${bright}Timeout: The CSI plugin did not become ready within the expected time.${reset}"
    exit 1
  fi
  NUM_SPEC=$(kubectl get statefulset csi-hostpathplugin  -o jsonpath='{.spec.replicas}')
  NUM_STATUS=$(kubectl get statefulset csi-hostpathplugin -o jsonpath='{.status.availableReplicas}')
  if [[ "$NUM_SPEC" == "$NUM_STATUS" ]]; then
    echo "${bright}Success: The CSI plugin is deployed and ready.${reset}"
    break
  fi
  sleep 1
  ((++ITER))
done
