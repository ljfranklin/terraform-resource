FROM golang:1.6

# install apt deps
RUN apt-get -y update && \
    apt-get -y install unzip && \
    apt-get clean

# install terraform
RUN wget -O /tmp/terraform.zip https://releases.hashicorp.com/terraform/0.6.16/terraform_0.6.16_linux_amd64.zip && \
    unzip /tmp/terraform.zip -d /usr/local/bin/ && \
    rm /tmp/terraform.zip

# build resource
COPY ./src/ /go/src/
WORKDIR /go
RUN go build -o /opt/resource/check terraform-resource/cmd/check && \
    go build -o /opt/resource/in terraform-resource/cmd/in && \
    go build -o /opt/resource/out terraform-resource/cmd/out
