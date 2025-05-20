#!/bin/bash
REPOSITORY=ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-k8s-operator
TAG=preview
make build
docker build -t ${REPOSITORY}:${TAG} .
docker push ${REPOSITORY}:${TAG}