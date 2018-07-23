# Deployment Tool 
a.k.a. CPSwarm Bulk Deployment Tool

This repository consists of the following packages:
* manager: The Deployment Manager responsible for propagating deployment tasks
* agent: The Deployment Agent responsible for performing tasks and reporting results
* model: Data models used among other packages

## Development Status
| Feature                          | Functional |
|----------------------------------|:----------:|
| Deployment Logs                  | ✔          |
| Package Assembly                 | -          |
| Package Transfer                 | ✔          |
| Package Installation             | ✔          |
| Validation Testing               | -          |
| Package Activation               | ✔          |
| Certificate Manager              | -          |
| Package Validation               | -          |
| Tamper Detection                 | -          |

### Dependencies
* [ZeroMQ v4.x.x](http://zeromq.org/intro:get-the-software)

**Install via apt**
```
sudo apt install libzmq5
```

## Compile from source
### Get the codes
```
git clone <repo-address> deployment-tool/src/code.linksmart.eu/dt/deployment-tool
```

### Build
```
export GOPATH=$(pwd)
go install code.linksmart.eu/dt/deployment-tool/manager
go install code.linksmart.eu/dt/deployment-tool/agent
```
