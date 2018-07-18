FROM farshidtz/zeromq-armv7:go

WORKDIR /home

COPY temp src/code.linksmart.eu/dt/deployment-tool/

ENV GOPATH=/home

# the armv7 compilation on a macOS host only works during build time:
RUN go install code.linksmart.eu/dt/deployment-tool/agent