#!/bin/bash -e

pushd /tmp > /dev/null
go get -u github.com/mattn/goveralls
go get -u honnef.co/go/tools/cmd/staticcheck
go get -u golang.org/x/lint/golint
go get -u github.com/client9/misspell/cmd/misspell
popd > /dev/null
