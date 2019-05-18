#!/bin/sh -x
set -e

version=$1
name=linksmart-deployment-agent

echo $version $name

src=$name/usr/local/src/code.linksmart.eu/dt/deployment-tool
git clone https://github.com/cpswarm/deployment-tool.git $src
rm -fr $src/ui $src/.git $src/examples

mkdir -p $name/DEBIAN
mkdir -p $name/lib/systemd/system
mkdir -p $name/var/local/$name

# control file, pre and post scripts
cp control preinst postinst $name/DEBIAN/
sed -i "s/<ver>/${version}/g" $name/DEBIAN/control
# service file
cp service $name/lib/systemd/system/$name.service

dpkg-deb --build $name