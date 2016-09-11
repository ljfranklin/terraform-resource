FROM alpine:latest

RUN apk update && \
    apk add ca-certificates git

COPY terraform/* /usr/local/bin/
COPY check in out /opt/resource/
