#!/bin/sh
echo "================================================================================"
echo "========================== DEPLOYMENT AGENT INSTALLER =========================="
# check arguments
if [ -z "$1" ]
then
	echo "Empty argument.\n Please provide environment key-values separated by spaces. E.g.:"
	echo "MANAGER_HOST=example.com MANAGER_PUBLIC_KEY_STR=keystring TAGS=linux,arm"
	exit 1
fi

serviceName=linksmart-deployment-agent
wd=/var/local/$serviceName
packageName=deployment-agent-linux-arm.deb
downloadURL=https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/linux-arm-debian-package/$packageName
envFile=.env

echo "================================================================================"
echo "Removing old files:\n"
ls $packageName*
rm $packageName*
ls $wd/.env
rm $wd/.env

set -e

echo "\n================================================================================"
echo "Writing variables to $wd/$envFile:\n"
mkdir -p $wd
for keyval in "$@"
do
    echo "$keyval"
    echo "$keyval" >> $wd/.env
done

echo "\n================================================================================"
echo "Downloading and installing the debian package:\n"
wget $downloadURL
apt install ./$packageName

echo "\n================================================================================"
# This will fail if the key name is different
echo "DONE! Add the public key to manager ($wd/agent.pub):\n"
cat $wd/agent.pub
echo "\n"
