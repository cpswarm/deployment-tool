#!/bin/sh
echo "DEPLOYMENT AGENT INSTALLER"

# check arguments
if [ -z "$1" ]
then
	echo "Empty argument.\n Please provide environment key-values separated by spaces. E.g. MANAGER_HOST=example.com MANAGER_PUBLIC_KEY_STR=keystring"
	exit 1
fi

wd=/var/local/linksmart-deployment-agent
echo "\nRemoving old files..."
echo "==============================================="
ls deployment-agent-linux-arm.deb*
rm deployment-agent-linux-arm.deb*
ls $wd/.env
rm $wd/.env

set -e

echo "\nDownloading and installing the debian package:"
echo "==============================================="
wget https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/linux-arm-debian-package/deployment-agent-linux-arm.deb
apt install ./deployment-agent-linux-arm.deb

echo "\nWriting variables to $wd/.env:"
echo "==============================================="
mkdir -p $wd
for var in "$@"
do
    echo "$var"
    echo "$var" >> $wd/.env
done

echo "\nGenerating key pair for agent:"
echo "==============================================="
/usr/local/bin/linksmart-deployment-agent -newkeypair $wd/agent

echo "\nDone. \nAdd the public key to manager:"
cat $wd/agent.pub
echo ""