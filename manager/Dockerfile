FROM ubuntu:bionic

WORKDIR /home

ADD https://pipelines.linksmart.eu/artifact/CPSW-DTB/shared/build-latest/linux-amd64-binary-distributions/deployment-manager-linux-amd64 .
RUN chmod +x deployment-manager-linux-amd64

ADD https://pipelines.linksmart.eu/browse/CPSW-DTB-101/artifact/shared/Frontend-UI/deployment-tool-ui.tar .
RUN tar xvf deployment-tool-ui.tar

ENV PRIVATE_KEY=/home/keys/manager.key
ENV AUTHORIZED_KEYS=/home/keys/authorized_keys

VOLUME /home/keys
EXPOSE 5556 5557 8080

CMD ["./deployment-manager-linux-amd64"]

# Run:
# docker run -p 8080:8080 -p 5556-5557:5556-5557 -v $(pwd)/keys:/home/keys --name=deployment-manager --restart=unless-stopped -d farshidtz/deployment-manager