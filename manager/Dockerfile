FROM ubuntu:bionic

WORKDIR /home

ADD https://pipelines.linksmart.eu/artifact/CPSW-DTB/shared/build-latest/linux-amd64-binary-distributions/deployment-manager-linux-amd64 .
RUN chmod +x deployment-manager-linux-amd64

ENV PRIVATE_KEY=/home/keys/manager.key
ENV PUBLIC_KEY=/home/keys/manager.pub

VOLUME /home/keys
EXPOSE 5556 5557 8080

ENTRYPOINT ["./deployment-manager-linux-amd64"]

# Generate key pair:
# docker run --rm -v $(pwd)/volumes/keys:/home/keys linksmart/deployment-manager -newkeypair keys/manager
# Run:
# docker run -p 8080:8080 -p 5556-5557:5556-5557 -v $(pwd)/volumes/keys:/home/keys --name=deployment-manager --restart=unless-stopped -d linksmart/deployment-manager