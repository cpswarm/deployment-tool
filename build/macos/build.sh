#!/bin/sh

# cleanup old things
docker rm agent-armv7
docker rmi agent-armv7
rm -fr temp

set -e

echo "Copying the code..."
mkdir -p temp bin
cp -R ../../agent temp
cp -R ../../model temp
cp -R ../../vendor temp

echo "Compiling..."
docker build -t agent-armv7 .
docker run --name agent-armv7 agent-armv7
docker cp agent-armv7:/home/bin/agent bin/agent-linux-armv7

echo "Cleaning up..."
docker rm agent-armv7
docker rmi agent-armv7
rm -fr temp

echo "Success!"