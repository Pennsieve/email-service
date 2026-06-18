#! /bin/bash
set -e

echo ""
echo "*******************************"
echo "*   Testing Email Service     *"
echo "*******************************"
echo ""
go test -v ./...
