#!/bin/bash -e
# Run from directory above via ./scripts/check.sh

echo "Checking formatting..."
if [ -n "$(gofmt -s -l .)" ]; then
    echo "Code is not formatted. Run 'gofmt -s -w .'"
    exit 1
fi
echo "Checking with go vet..."
go vet ./...
echo "Checking with staticcheck..."
staticcheck ./...
echo "Checking with golint..."
golint -set_exit_status ./...
echo "Checking with misspell..."
misspell -error -locale US .
