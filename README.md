# CPSwarm Deployment Tool 
An over-the-air (OTA) software deployment tool for IoT applications. This project is currently under active development and not ready for production.

## Documentation
* [Github Wiki](https://github.com/cpswarm/deployment-tool/wiki)

## Development Status
| Feature                          | Functional |
|----------------------------------|:----------:|
| Deployment Logs                  | ✔          |
| Package Assembly                 | ✔          |
| Package Transfer                 | ✔          |
| Package Installation             | ✔          |
| Validation Testing               | -          |
| Package Activation               | ✔          |
| Certificate Manager              | -          |
| Package Validation               | -          |
| Tamper Detection                 | -          |


## Dependencies
* [ZeroMQ v4.x.x](http://zeromq.org/intro:get-the-software).   
Runtime: libzmq5, Build: libzmq3-dev


## Install (Debian)
```
wget https://pipelines.linksmart.eu/browse/CPSW-DTB/latest/artifact/shared/Debian-Package/deployment-agent-linux-arm.deb
sudo apt install ./deployment-agent-linux-arm.deb
```

### Run after boot
```
sudo systemctl enable linksmart-deployment-agent
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

#### Build with static linked dependencies (armv7)
```
sudo sh build/zeromq/install-armv7.sh
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lsodium -lrt -lstdc++ -lm -lc -lgcc" go build -v --ldflags '-extldflags "-static"' -a code.linksmart.eu/dt/deployment-tool/agent
```

[![HitCount](http://hits.dwyl.io/cpswarm/deployment-tool.svg)](http://hits.dwyl.io/cpswarm/deployment-tool)
