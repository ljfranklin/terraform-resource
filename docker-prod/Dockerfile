FROM alpine:latest

RUN apk update && \
    apk add ca-certificates git bash

COPY terraform/* /usr/local/bin/
COPY check in out /opt/resource/
