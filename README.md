# Deployment Tool 
a.k.a. CPSwarm Bulk Deployment Tool

This repository consists of the following packages:
* manager: The Deployment Manager responsible for propagating deployment tasks
* agent: The Deployment Agent responsible for performing tasks and reporting results
* model: Data models used among other packages

## Compile from source
### Dependencies
* [ZeroMQ v4.x.x](http://zeromq.org/intro:get-the-software)

### Get the codes
```
git clone <repo-address> src/code.linksmart.eu/dt/deployment-tool
```

### Build
```
export GOPATH=$(pwd)
go install code.linksmart.eu/dt/deployment-tool/manager
go install code.linksmart.eu/dt/deployment-tool/agent
```
