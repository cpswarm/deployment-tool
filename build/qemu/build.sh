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
docker run --rm -v $(pwd)/temp:/home/src -v $(pwd)/bin:/home/bin --env GOPATH=/home -it farshidtz/zeromq:multiarch-ubuntu-core-armhf-xenial-go go install -v $package/agent

echo "Cleaning up..."
rm -fr temp

echo "Success!"
