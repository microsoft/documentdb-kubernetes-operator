#!/bin/bash
VERSION=1
helm package documentdb-chart --version 0.0.${VERSION}
helm install documentdb-operator ./documentdb-operator-0.0.${VERSION}.tgz