FROM farshidtz/zeromq:arm32v7-debian-go

WORKDIR /home

COPY temp src/code.linksmart.eu/dt/deployment-tool/

ENV GOPATH=/home

# the armv7 compilation on a macOS host only works during build time:
RUN go install -v code.linksmart.eu/dt/deployment-tool/agent