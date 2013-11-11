#!/bin/sh

export GOPATH=$(pwd)
cd $(pwd)
go get github.com/russross/blackfriday
go build golog.go
echo "Done"
