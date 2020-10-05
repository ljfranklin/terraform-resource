FROM alpine:latest

RUN apk update && \
    apk add ca-certificates git bash openssh-client

RUN wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub && \
  wget -q -O /tmp/glibc.apk https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.32-r0/glibc-2.32-r0.apk && \
  apk add /tmp/glibc.apk && \
  rm -rf /tmp/glibc.apk

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

RUN mkdir -p /usr/local/share/terraform/plugins/github.com/ashald/stateful/1.1.0/linux_amd64/ && \
  wget -O /usr/local/share/terraform/plugins/github.com/ashald/stateful/1.1.0/linux_amd64/terraform-provider-stateful_v1.1.0 \
    "https://github.com/ashald/terraform-provider-stateful/releases/download/v1.1.0/terraform-provider-stateful_v1.1.0-linux-amd64" && \
  chmod +x /usr/local/share/terraform/plugins/github.com/ashald/stateful/1.1.0/linux_amd64/terraform-provider-stateful_v1.1.0
COPY terraform/* /usr/local/bin/
COPY check in out /opt/resource/
