#!/usr/bin/env bash

cd "$(dirname "$0")/.." || exit

# Compile the plugin
CGO_ENABLED=0 go build -o bin/cnpg-i-wal-replica main.go
