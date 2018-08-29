#!/bin/sh
set -e

if [ -z "$1" ]
then
      echo "name of executable not given as argument."
      exit 1
fi

name=linksmart-deployment-agent

mv $1 $name.bin

mkdir -p $name/DEBIAN
mkdir -p $name/lib/systemd/system
mkdir -p $name/usr/local/bin
mkdir -p $name/var/local/$name

cp control $name/DEBIAN/
cp service $name/lib/systemd/system/$name.service
mv $name.bin $name/usr/local/bin/$name

dpkg-deb --build $name
mv $name.deb $1.deb