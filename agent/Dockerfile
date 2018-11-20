FROM ubuntu:bionic

WORKDIR /home

ADD https://pipelines.linksmart.eu/artifact/CPSW-DTB/shared/build-latest/linux-amd64-binary-distributions/deployment-agent-linux-amd64 .
RUN chmod +x deployment-agent-linux-amd64

ENV PRIVATE_KEY=/home/keys/agent.key
ENV PUBLIC_KEY=/home/keys/agent.pub
ENV MANAGER_PUBLIC_KEY=/home/keys/manager.pub

VOLUME /home/keys

ENTRYPOINT ["./deployment-agent-linux-amd64"]

# Run:
# docker run -p 8080:8080 -p 5556-5557:5556-5557 -v $(pwd)/keys:/home/keys --name=deployment-manager --restart=unless-stopped -d farshidtz/deployment-manager