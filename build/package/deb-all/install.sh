#!/bin/sh
echo "================================================================================"
echo "========================== DEPLOYMENT AGENT INSTALLER =========================="

# check arguments
if [ -z "$1" ]
then
	echo "Missing arguments. Provide environment key-values separated by spaces. E.g.:"
	echo "MANAGER_ADDR=example.com AUTH_TOKEN=keystring TAGS=linux,arm"
	exit 1
fi

serviceName=linksmart-deployment-agent
wd=/var/local/$serviceName
packageName=linksmart-deployment-agent.deb
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
echo "Done!\n"

echo "The service is managed by systemctl. Useful commands:\n"
echo "service $serviceName (status|start|stop|restart)"
echo "journalctl -n 100 -f -u $serviceName"
