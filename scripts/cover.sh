#!/bin/bash -e
# Run from directory above via ./scripts/cover.sh

go test -v -covermode=atomic -coverprofile=./cover.out -coverpkg=./server/... ./test
go tool cover -html=cover.out
