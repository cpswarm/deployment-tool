# CPSwarm Deployment Tool 
[![GoDoc](https://godoc.org/github.com/cpswarm/deployment-tool?status.svg)](https://godoc.org/github.com/cpswarm/deployment-tool)
[![Docker Pulls](https://img.shields.io/docker/pulls/linksmart/deployment-manager.svg)](https://hub.docker.com/r/linksmart/deployment-manager/tags)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/cpswarm/deployment-tool.svg)](https://github.com/cpswarm/deployment-tool/tags)
[![Build Status](https://travis-ci.com/cpswarm/deployment-tool.svg?branch=master)](https://travis-ci.com/cpswarm/deployment-tool)  

The CPSwarm Deployment Tool is a lightweight software deployment system for IoT devices. It aims to provide secure, practical, and easy to use utilities for over-the-air (OTA) provisioning of software on single-board computers (e.g. Raspberry Pi). The project is currently under active development.

![CPSwarm Deployment Tool - Conceptual Diagram](https://raw.githubusercontent.com/wiki/cpswarm/deployment-tool/figures/deployment-tool-concept-v3.jpg)

## Getting Started
* Documentation: [wiki](https://github.com/cpswarm/deployment-tool/wiki) | [apidocs](https://app.swaggerhub.com/apis-docs/farshidtz8/deployment-tool)
* :star: Deployment GUI: [source code](https://github.com/cpswarm/deployment-tool-ui) | [wiki](https://github.com/cpswarm/deployment-tool-ui/wiki)

## Deployment
Packages are built continuously with [Bamboo](https://pipelines.linksmart.eu/browse/CPSW-DTB/latest).

### Docker
Docker compose scripts are available for [Deployment Manager](https://github.com/cpswarm/deployment-tool/blob/master/manager/docker-compose.yml) and dummy [Deployment Agents](https://github.com/cpswarm/deployment-tool/blob/master/agent/docker-compose.yml).
### Install on Debian ARM
```bash
wget https://pipelines.linksmart.eu/artifact/CPSW-DTB/shared/build-latest/Debian-packages/linksmart-deployment-agent.deb
sudo apt install ./deployment-agent-linux-arm.deb
```

### Compile from source
Within the root of the repository:
```bash
go build -o bin/manager ./manager
go build -o bin/agent  ./agent
```
#### Build with static linking
```bash
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lrt -lstdc++ -lm -lc -lgcc" go build -v --ldflags '-extldflags "-static"' -a -o bin/agent ./agent
```
#### Compile using Go < 1.11
```bash
git clone <repo-addr> src/code.linksmart.eu/dt/deployment-tool
export GOPATH=$(pwd)
go build -v code.linksmart.eu/dt/deployment-tool/agent
```

## Development
### Run tests
Locally:
```bash
 go test ./tests -v -failfast
```
In a docker container:
```bash
docker network create test-network
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $(pwd):$(pwd) -w $(pwd) --network=test-network -e EXTERNAL-NETWORK=test-network golang:1.12 go test ./tests -v -failfast
docker network remove test-network
```

### Dependencies
* [ZeroMQ v4.x.x](http://zeromq.org/intro:get-the-software).  
Runtime: libzmq5, Build: libzmq3-dev

## Contributing
Contributions are welcome. 

Please fork, make your changes, and submit a pull request. For major changes, please open an issue first and discuss it with the other authors.

## Affiliation
![CPSwarm](https://github.com/cpswarm/template/raw/master/cpswarm.png)  
This work is supported by the European Commission through the [CPSwarm H2020 project](https://cpswarm.eu) under grant no. 731946.
