#!/bin/sh
echo "================================================================================"
echo "========================== DEPLOYMENT AGENT INSTALLER =========================="
# check arguments
if [ -z "$1" ]
then
	echo "Empty argument.\n Please provide environment key-values separated by spaces. E.g. MANAGER_HOST=example.com MANAGER_PUBLIC_KEY_STR=keystring"
	exit 1
fi

wd=/var/local/linksmart-deployment-agent
downloadURL=https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/linux-arm-debian-package/deployment-agent-linux-arm.deb
packageName=deployment-agent-linux-arm.deb
envFile=.env
serviceName=linksmart-deployment-agent

echo "\n================================================================================"
echo "Removing old files:\n"
ls $packageName*
rm $packageName*
ls $wd/.env
rm $wd/.env

set -e

echo "\n================================================================================"
echo "Downloading and installing the debian package:\n"
wget $downloadURL
apt install ./$packageName

echo "\n================================================================================"
echo "Writing variables to $wd/$envFile:\n"
mkdir -p $wd
for var in "$@"
do
    echo "$var"
    echo "$var" >> $wd/.env
done

echo "\n================================================================================"
echo "Generating key pair for agent:\n"
/usr/local/bin/$serviceName -newkeypair $wd/agent

echo "\n================================================================================"
echo "Restarting service...\n"
service $serviceName restart

echo "DONE! Add the public key to manager:\n"
cat $wd/agent.pub
echo "\n"

