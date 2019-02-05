FROM farshidtz/zeromq:golang-linux-amd64-stretch

COPY temp /home/src/code.linksmart.eu/dt/deployment-tool
WORKDIR /home

RUN go build -o deployment-agent-linux-amd64 code.linksmart.eu/dt/deployment-tool/agent

RUN ./deployment-agent-linux-amd64 -newkeypair agent

ENV PRIVATE_KEY=agent.key
ENV PUBLIC_KEY=agent.pub

ENTRYPOINT ["./deployment-agent-linux-amd64"]

# BUILD
# docker build -t agent-sa .