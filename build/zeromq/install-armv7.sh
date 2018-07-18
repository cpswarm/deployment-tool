#!/bin/sh

ZEROMQ_VER=4.2.3

# exit if a command exists with non-zero code
set -e

#
# INSTALL DEPENDENCIES
#
apt -y install libtool pkg-config build-essential autoconf automake

#echo 'Installing libsodium'
## https://download.libsodium.org/libsodium/releases/
#LIBSODIUM_VER=1.0.16
#wget https://download.libsodium.org/libsodium/releases/libsodium-$LIBSODIUM_VER.tar.gz
#tar -zxvf libsodium-$LIBSODIUM_VER.tar.gz
#cd libsodium-$LIBSODIUM_VER
#./configure
#make
#sudo make install
#cd ..
apt -y install libsodium18

#
# INSTALL ZEROMQ
#
echo 'Installing ZeroMQ'
wget https://github.com/zeromq/libzmq/releases/download/v$ZEROMQ_VER/zeromq-$ZEROMQ_VER.tar.gz
tar -zxvf zeromq-$ZEROMQ_VER.tar.gz
cd zeromq-$ZEROMQ_VER
./configure
make
make install
ldconfig