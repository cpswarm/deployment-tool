# CPSwarm Deployment Tool 
[![GoDoc](https://godoc.org/github.com/cpswarm/deployment-tool?status.svg)](https://godoc.org/github.com/cpswarm/deployment-tool)
[![Go Report Card](https://goreportcard.com/badge/github.com/cpswarm/deployment-tool)](https://goreportcard.com/report/github.com/cpswarm/deployment-tool)
[![Docker Pulls](https://img.shields.io/docker/pulls/linksmart/deployment-manager.svg)](https://hub.docker.com/r/linksmart/deployment-manager/tags)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/cpswarm/deployment-tool.svg)](https://github.com/cpswarm/deployment-tool/tags)  

The CPSwarm Deployment Tool is a lightweight software deployment system for IoT devices. It aims to provide secure, practical, and easy to use utilities for over-the-air (OTA) provisioning of software on single-board computers (e.g. Raspberry Pi). The project is currently under active development.

![CPSwarm Deployment Tool - Conceptual Diagram](https://raw.githubusercontent.com/wiki/cpswarm/deployment-tool/figures/deployment-tool-concept-v3.jpg)

## Links
* Documentation: [wiki](https://github.com/cpswarm/deployment-tool/wiki) | [apidocs](https://app.swaggerhub.com/apis-docs/farshidtz8/deployment-tool)
* :star: Deployment GUI: [source code](https://github.com/cpswarm/deployment-tool-ui) | [wiki](https://github.com/cpswarm/deployment-tool-ui/wiki)

## Development Status
- [x] Graphical User Interface ([separate repo](https://github.com/cpswarm/deployment-tool-ui))
- [x] Package Build
- [x] Package Transfer
- [x] Package Installation
- [x] Package Execution
- [x] Key Management
- [ ] Tamper Detection


## Install
Packages are built continuously [here](https://pipelines.linksmart.eu/browse/CPSW-DTB/latest).
### Docker
Docker compose scripts are available for [Deployment Manager](https://github.com/cpswarm/deployment-tool/blob/update-readme/manager/docker-compose.yml) and dummy [Deployment Agents](https://github.com/cpswarm/deployment-tool/blob/update-readme/agent/docker-compose.yml).
### Install on Debian ARM
```bash
wget https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/Debian-Package/deployment-agent-linux-arm.deb
sudo apt install ./deployment-agent-linux-arm.deb
```

## Compile from source
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

## Dependencies
* [ZeroMQ v4.x.x](http://zeromq.org/intro:get-the-software).  
Runtime: libzmq5, Build: libzmq3-dev
