#!/bin/bash
set -e

GOOS=linux   GOARCH=amd64  go build -ldflags="-s -w" -o cost-estimator-linux main.go
GOOS=linux   GOARCH=arm64  go build -ldflags="-s -w" -o cost-estimator-linux-arm64 main.go
GOOS=darwin  GOARCH=amd64  go build -ldflags="-s -w" -o cost-estimator-macos main.go
GOOS=darwin  GOARCH=arm64  go build -ldflags="-s -w" -o cost-estimator-macos-arm64 main.go
GOOS=windows GOARCH=amd64  go build -ldflags="-s -w" -o cost-estimator.exe main.go

echo "Binaries built:"
ls -lh cost-estimator*
