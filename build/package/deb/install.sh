#!/bin/sh
set -e
echo "INSTALLING DEPLOYMENT AGENT AS A SERVICE"

# check arguments
if [ -z "$1" ]
then
	echo "Empty argument.\n Please provide environment key-values separated by spaces. E.g. MANAGER_HOST=example.com MANAGER_PUBLIC_KEY_STR=keystring"
	exit 1
fi

echo "\nDownloading and installing the debian package:"
echo "==============================================="
rm deployment-agent-linux-arm.deb*
wget https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/linux-arm-debian-package/deployment-agent-linux-arm.deb
apt install ./deployment-agent-linux-arm.deb
wd=/var/local/linksmart-deployment-agent

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