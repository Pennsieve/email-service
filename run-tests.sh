#! /bin/bash
set -e

echo ""
echo "**********************************"
echo "*   Testing Service Lambda       *"
echo "**********************************"
echo ""
cd ./lambda/service; \
  go test -v ./... ; \
  cd ../..
echo ""
echo "*******************************"
echo "*   Testing Queue Lambda      *"
echo "*******************************"
echo ""
cd ./lambda/queue; \
  go test -v ./... ; \
  cd ../..
