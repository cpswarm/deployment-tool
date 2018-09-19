FROM golang:stretch

ARG ZEROMQ_VER=4.2.3

RUN apt update && apt install -y wget libtool pkg-config build-essential autoconf automake libsodium-dev

RUN wget https://github.com/zeromq/libzmq/releases/download/v$ZEROMQ_VER/zeromq-$ZEROMQ_VER.tar.gz && \
    tar -zxvf zeromq-$ZEROMQ_VER.tar.gz && \
    cd zeromq-$ZEROMQ_VER && \
    ./configure && \
    make && \
    make install && \
    ldconfig
RUN rm -fr /home/zeromq-$ZEROMQ_VER*

ENV GOPATH=/home
WORKDIR ${GOPATH}

# docker build -t farshidtz/zeromq:golang-linux-amd64-stretch .

#WORKDIR /home
#ENV BUILDDIR src/code.linksmart.eu/dt/deployment-tool
#RUN mkdir -p ${BUILDDIR}
#COPY manager ${BUILDDIR}/manager
#COPY model ${BUILDDIR}/model
#COPY vendor ${BUILDDIR}/vendor

#ENV GOPATH=/home
#RUN go install -v code.linksmart.eu/dt/deployment-tool/manager

#RUN CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lsodium -lrt -lstdc++ -lm -lc -lgcc" go build -v --ldflags '-extldflags "-static"' \
#    -o bin/deployment-manager-static-linux-amd64 code.linksmart.eu/dt/deployment-tool/manager

#CMD bash
