#!/bin/sh

# cleanup old things
rm -fr temp

set -e

echo "Copying the code..."
mkdir -p temp
cp -rv ../../manager temp/
cp -rv ../../agent temp/
cp -r ../../vendor temp/

docker build -t agent-sa .

echo "Cleaning up..."
rm -fr temp

echo "Success!"

