#!/bin/bash

# GoSentry test runner
# Runs go vet and go test with race detection

set -e

echo "Running go vet..."
go vet ./...

echo ""
echo "Running go test with race detection..."
go test -race ./...

echo ""
echo "✓ All tests passed"
