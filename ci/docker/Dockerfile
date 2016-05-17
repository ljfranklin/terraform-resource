FROM golang:1.6

# install apt deps
RUN apt-get -y update && \
    apt-get -y install unzip && \
    apt-get clean

# install go deps
RUN go get github.com/onsi/ginkgo/ginkgo
ENV PATH=$PATH:/go/bin

# install terraform
RUN wget -O /tmp/terraform.zip https://releases.hashicorp.com/terraform/0.6.16/terraform_0.6.16_linux_amd64.zip && \
    unzip /tmp/terraform.zip -d /usr/local/bin/ && \
    rm /tmp/terraform.zip
