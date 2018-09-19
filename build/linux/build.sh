#!/bin/sh

# cleanup old things
rm -fr temp

set -e

echo "Copying the code..."
package=code.linksmart.eu/dt/deployment-tool
mkdir -p temp/$package bin
cp -rv ../../manager temp/$package
cp -rv ../../model temp/$package
cp -rv ../../vendor temp/$package

echo "Compiling... (IF HUNG, KILL THE CONTAINER!)"
docker run --rm -v $(pwd)/temp:/home/src -v $(pwd)/bin:/home/bin -v $(pwd)/static-build.sh:/home/cmd.sh farshidtz/zeromq:golang-linux-amd64-stretch sh cmd.sh

echo "Cleaning up..."
rm -fr temp

echo "Success!"

