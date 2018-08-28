#!/bin/sh

#wget https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/NB/Binary-Distributions/deployment-agent-linux-arm

mkdir -p linksmart-deployment-agent/DEBIAN
mkdir -p linksmart-deployment-agent/lib/systemd/system
mkdir -p linksmart-deployment-agent/usr/local/bin
mkdir -p linksmart-deployment-agent/var/local/linksmart-deployment-agent

cp control linksmart-deployment-agent/DEBIAN/
cp linksmart-deployment-agent.service linksmart-deployment-agent/lib/systemd/system/
mv deployment-agent-linux-arm linksmart-deployment-agent/usr/local/bin/

dpkg-deb --build linksmart-deployment-agent