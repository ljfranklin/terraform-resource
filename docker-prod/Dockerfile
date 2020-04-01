FROM alpine:latest

RUN apk update && \
    apk add ca-certificates git bash libc6-compat openssh-client

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

RUN wget -O /usr/local/bin/terraform-provider-stateful_v1.1.0-linux-amd64 \
  "https://github.com/ashald/terraform-provider-stateful/releases/download/v1.1.0/terraform-provider-stateful_v1.1.0-linux-amd64" && \
  chmod +x /usr/local/bin/terraform-provider-stateful_v1.1.0-linux-amd64
RUN wget -O /tmp/terraform-provider-vinyldns-linux_amd64-0.9.4.tar.gz \
  "https://github.com/vinyldns/terraform-provider-vinyldns/releases/download/0.9.4/release.terraform-provider-vinyldns-linux_amd64-0.9.4.tgz" && \
  tar -C /usr/local/bin -xzvf /tmp/terraform-provider-vinyldns-linux_amd64-0.9.4.tar.gz
COPY terraform/* /usr/local/bin/
COPY check in out /opt/resource/
