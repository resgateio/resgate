#!/bin/bash -e
# Run from directory above via ./scripts/cover.sh

go test -v -covermode=atomic -coverprofile=./cover.out -coverpkg=./server/... ./test

# If we have an arg, assume travis run and push to coveralls. Otherwise launch browser results
if [[ -n $1 ]]; then
    $HOME/gopath/bin/goveralls -coverprofile=cover.out -service travis-ci
    rm -rf ./cover.out
else
    go tool cover -html=cover.out
fi
