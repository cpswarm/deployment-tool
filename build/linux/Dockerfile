FROM golang:stretch

ARG ZEROMQ_VER=4.3.2
LABEL ZEROMQ_VER=$ZEROMQ_VER

RUN apt update && apt install -y wget libtool pkg-config build-essential autoconf automake libsodium-dev

RUN wget https://github.com/zeromq/libzmq/releases/download/v$ZEROMQ_VER/zeromq-$ZEROMQ_VER.tar.gz && \
    tar -zxvf zeromq-$ZEROMQ_VER.tar.gz && \
    cd zeromq-$ZEROMQ_VER && \
    ./configure && \
    make && \
    make install && \
    ldconfig
RUN rm -fr /home/zeromq-$ZEROMQ_VER*

WORKDIR /home

# Build:
# docker build -t farshidtz/zeromq:golang-linux-amd64-stretch .