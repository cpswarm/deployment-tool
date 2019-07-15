# CPSwarm Deployment Tool 
[![GoDoc](https://godoc.org/github.com/cpswarm/deployment-tool?status.svg)](https://godoc.org/github.com/cpswarm/deployment-tool)
[![Go Report Card](https://goreportcard.com/badge/github.com/cpswarm/deployment-tool)](https://goreportcard.com/report/github.com/cpswarm/deployment-tool)
[![Docker Pulls](https://img.shields.io/docker/pulls/linksmart/deployment-manager.svg)](https://hub.docker.com/r/linksmart/deployment-manager/tags)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/cpswarm/deployment-tool.svg)](https://github.com/cpswarm/deployment-tool/tags)  
[![Build Status](https://pipelines.linksmart.eu/plugins/servlet/wittified/build-status/CPSW-DTB)](https://pipelines.linksmart.eu/browse/CPSW-DTB/latest)

An over-the-air (OTA) software deployment tool for IoT applications. This project is currently under active development and not ready for production.

## Links
* [Documentation](https://github.com/cpswarm/deployment-tool/wiki)
* [Development Tool UI](https://github.com/cpswarm/deployment-tool-ui)

## Development Status
- [x] Package Build
- [x] Package Transfer
- [x] Package Installation
- [x] Package Runtime
- [x] Key Management
- [ ] Tamper Detection


## Dependencies
* [ZeroMQ v4.x.x](http://zeromq.org/intro:get-the-software).   
Runtime: libzmq5, Build: libzmq3-dev


## Install (Debian ARM)
```bash
wget https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/Debian-Package/deployment-agent-linux-arm.deb
sudo apt install ./deployment-agent-linux-arm.deb
```

## Compile from source

### Build
Within the root of the repository:
```bash
go build -o bin/manager ./manager
go build -o bin/agent  ./agent
```

#### Using Go < 1.11
```bash
git clone <repo-addr> src/code.linksmart.eu/dt/deployment-tool
export GOPATH=$(pwd)
go build -v code.linksmart.eu/dt/deployment-tool/agent
```

#### Build with static linking
```bash
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lrt -lstdc++ -lm -lc -lgcc" go build -v --ldflags '-extldflags "-static"' -a -o bin/agent ./agent
```

