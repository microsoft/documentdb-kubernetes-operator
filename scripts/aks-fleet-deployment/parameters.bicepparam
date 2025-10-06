using './main.bicep'

param hubClusterName = 'aks-fleet-hub'
param hubRegion = 'eastus2'
param memberRegions = [
  'westus3'
  'uksouth'
  'eastus2'
]
param kubernetesVersion = ''
param nodeCount = 1
param hubVmSize = 'Standard_DS2_v2'
