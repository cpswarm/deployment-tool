#!/bin/sh

set -e

echo "BUILDING MANAGER"
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lsodium -lrt -lstdc++ -lm -lc -lgcc" \
go build -mod=vendor -v --ldflags '-extldflags "-static"' -o bin/deployment-manager-linux-amd64 ./manager

echo "BUILDING AGENT"
CGO_CPPFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lzmq -lpthread -lsodium -lrt -lstdc++ -lm -lc -lgcc" \
go build -mod=vendor -v --ldflags '-extldflags "-static"' -o bin/deployment-agent-linux-amd64 ./agent
