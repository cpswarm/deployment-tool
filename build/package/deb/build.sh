#!/bin/sh

if [ -z "$1" ]
then
      echo "name of executable not given as argument."
      exit 1
fi

set -e

mv $1 deployment-agent

mkdir -p linksmart-deployment-agent/DEBIAN
mkdir -p linksmart-deployment-agent/lib/systemd/system
mkdir -p linksmart-deployment-agent/usr/local/bin
mkdir -p linksmart-deployment-agent/var/local/linksmart-deployment-agent

cp control linksmart-deployment-agent/DEBIAN/
cp linksmart-deployment-agent.service linksmart-deployment-agent/lib/systemd/system/
mv deployment-agent linksmart-deployment-agent/usr/local/bin/

dpkg-deb --build linksmart-deployment-agent