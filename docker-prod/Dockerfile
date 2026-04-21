FROM alpine:latest

ARG TARGETARCH

RUN apk update && \
  apk add aws-cli ca-certificates git bash openssh-client curl sops kubectl
RUN aws --version
RUN kubectl version --client

RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking no" >> $HOME/.ssh/config
RUN echo "LogLevel quiet" >> $HOME/.ssh/config
RUN chmod 0600 $HOME/.ssh/config

RUN mkdir -p /usr/local/share/terraform/plugins/github.com/ashald/stateful/1.2.0/linux_${TARGETARCH}/ && \
  wget -O /usr/local/share/terraform/plugins/github.com/ashald/stateful/1.2.0/linux_${TARGETARCH}/terraform-provider-stateful_v1.2.0 \
  "https://github.com/ashald/terraform-provider-stateful/releases/download/v1.2.0/terraform-provider-stateful_v1.2.0-linux-${TARGETARCH}" && \
  chmod +x /usr/local/share/terraform/plugins/github.com/ashald/stateful/1.2.0/linux_${TARGETARCH}/terraform-provider-stateful_v1.2.0
COPY $TARGETARCH/terraform/* /usr/local/bin/
RUN tofu --version

ENV TG_TF_FORWARD_STDOUT=true

COPY $TARGETARCH/check $TARGETARCH/in $TARGETARCH/out /opt/resource/
