# Multi-Cluster Hub Controller Design

## Problem

* There is no simple way for the promotion token from a demoted cluster to 
transfer to the newly promoted cluster

## Implementation

This will be a seperate k8s operator running in the KubeFleet hub,
It will try to remain as minimal as possible.

### Promotion token management

The Controller will be able to query endpoints on the member clusters
with the promotion token, and then create a configMap and CRP to
send that token to the new primary cluster. It will have access to the 
documentdb crp so it will be able to see which member is primary. 

It will clean up the token and crp when the promotion is complete. 
It can determine this through another documentdb operator endpoint.

## Other possible additions

### Streamlined Operator and Cluster deployment

This new conrtoller could theoretically handle the installation and 
distribution of the cert manager and the operator to save the user from
having to deploy a large and cumbersome CRP. It could also monitor 
the DocumentDB CRD and automatically create a CRP for that matching
the provided clusterReplication field.

## Security considerations

This operator will have no more access than the fleet manager already
does, and the member cluster operator endpoints will be limited to the
least amount of information provided possible and only grant access 
to the fleet controller.

## Alternatives

Currently, we perform this promotion token transfer using a nginx pod
and a multi-cluster service when using KubeFleet 
