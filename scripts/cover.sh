#!/bin/bash -e
# Run from directory above via ./scripts/cover.sh

env RESGATE_TEST_EXTENDED=1 go test -v -covermode=atomic -coverprofile=./cover.out -coverpkg=./server/... ./...
go tool cover -html=cover.out
