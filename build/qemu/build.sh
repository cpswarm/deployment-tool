#!/bin/sh

# cleanup old things
rm -fr temp

set -e

echo "Copying the code..."
package=code.linksmart.eu/dt/deployment-tool
mkdir -p temp/$package
cp -rv ../../agent temp/$package
cp -rv ../../model temp/$package
cp -rv ../../vendor temp/$package

echo "Compiling... (IF HUNG, KILL THE CONTAINER!)"
docker run --rm -v $(pwd)/temp:/home/src -v $(pwd)/bin:/home/bin farshidtz/zeromq:multiarch-ubuntu-core-armhf-xenial-go \
    go build -v -o bin/agent-linux-arm $package/agent

echo "Cleaning up..."
rm -fr temp

echo "Success!"
