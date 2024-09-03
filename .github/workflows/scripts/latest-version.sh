#!/bin/bash

set -e 

go_latest="${{ github.event.inputs.go_sdk_version }}"
if [ -z "$go_latest" ]; then
go_latest=$(./temporal-features latest-sdk-version --lang go)
echo "Derived latest Go SDK release version: $go_latest"
fi
echo "go_latest=$go_latest" >> $GITHUB_OUTPUT

typescript_latest="${{ github.event.inputs.typescript_sdk_version }}"
if [ -z "$typescript_latest" ]; then
typescript_latest=$(./temporal-features latest-sdk-version --lang ts)
echo "Derived latest Typescript SDK release version: $typescript_latest"
fi
echo "typescript_latest=$typescript_latest" >> $GITHUB_OUTPUT

java_latest="${{ github.event.inputs.java_sdk_version }}"
if [ -z "$java_latest" ]; then
java_latest=$(./temporal-features latest-sdk-version --lang java)
echo "Derived latest Java SDK release version: $java_latest"
fi
echo "java_latest=$java_latest" >> $GITHUB_OUTPUT

python_latest="${{ github.event.inputs.python_sdk_version }}"
if [ -z "$python_latest" ]; then
python_latest=$(./temporal-features latest-sdk-version --lang py)
echo "Derived latest Python SDK release version: $python_latest"
fi
echo "python_latest=$python_latest" >> $GITHUB_OUTPUT

csharp_latest="${{ github.event.inputs.dotnet_sdk_version }}"
if [ -z "$csharp_latest" ]; then
csharp_latest=$(./temporal-features latest-sdk-version --lang cs)
echo "Derived latest Dotnet SDK release version: $csharp_latest"
fi
echo "csharp_latest=$csharp_latest" >> $GITHUB_OUTPUT
