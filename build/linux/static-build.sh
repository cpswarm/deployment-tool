#!/bin/sh

echo "BUILDING MANAGER"
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lsodium -lrt -lstdc++ -lm -lc -lgcc" \
go build -v --ldflags '-extldflags "-static"' -o bin/deployment-manager-linux-amd64 code.linksmart.eu/dt/deployment-tool/manager

echo "BUILDING AGENT"
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lsodium -lrt -lstdc++ -lm -lc -lgcc" \
go build -v --ldflags '-extldflags "-static"' -o bin/deployment-agent-linux-amd64 code.linksmart.eu/dt/deployment-tool/agent
