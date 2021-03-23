#!/bin/bash

mkdir tmp
cp ./dp tmp
cp ./script/Dockerfile tmp
cd ./tmp
docker build -t k8s-deviceplugin-example:test .
cd -
rm -rf tmp