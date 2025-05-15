#!/bin/bash
VERSION=1
helm uninstall documentdb-operator
rm -rf documentdb-operator-0.0.${VERSION}.tgz