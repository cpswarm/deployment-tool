#!/bin/sh
set -e

exec=$1
version=$2
name=linksmart-deployment-agent
arch=armhf

echo $exec $version $name $arch

mv $exec $name.bin

mkdir -p $name/DEBIAN
mkdir -p $name/lib/systemd/system
mkdir -p $name/usr/local/bin
mkdir -p $name/var/local/$name

# build control file
echo "Package: "$name >> control
echo "Version: "$version >> control
echo "Architecture: "$arch >> control
echo "Maintainer: LinkSmart®" >> control
echo "Description: LinkSmart® Deployment Agent" >> control

mv control $name/DEBIAN/
cp service $name/lib/systemd/system/$name.service
mv $name.bin $name/usr/local/bin/$name

dpkg-deb --build $name
mv $name.deb $exec.deb