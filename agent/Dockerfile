FROM ubuntu:bionic

WORKDIR /home

ADD https://pipelines.linksmart.eu/artifact/CPSW-DTB/shared/build-latest/linux-amd64-binary-distributions/deployment-agent-linux-amd64 .
RUN chmod +x deployment-agent-linux-amd64

ENV PRIVATE_KEY=agent.key
ENV PUBLIC_KEY=agent.pub

WORKDIR /home/agent
VOLUME /home/agent

ENTRYPOINT ["/home/deployment-agent-linux-amd64"]

# Generate key pair:
# docker run --rm -v $(pwd)/agent:/home/agent linksmart/deployment-agent -newkeypair agent
# Run:
# docker run -e MANAGER_ADDR=http://manager:8080 -e AUTH_TOKEN=<token> -v $(pwd)/agent:/home/agent linksmart/deployment-agent
