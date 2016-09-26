FROM ubuntu:16.04
MAINTAINER Jonas Finnemann Jensen <jopsen@gmail.com>

RUN apt-get update -y
RUN apt-get upgrade -y

# Install runtime dependencies
RUN apt-get install -y qemu-system-x86 qemu-utils dnsmasq-base liblz4-tool iptables

# Install build dependencies
RUN apt-get install -y git curl screen nano genisoimage build-essential

# Install golang 1.7
RUN curl -L https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz > /tmp/go.tar.gz \
 && tar -xvf /tmp/go.tar.gz -C /usr/local \
 && rm /tmp/go.tar.gz \
    ;

# Install zstd 1.0.0
RUN curl -L https://github.com/facebook/zstd/archive/v1.0.0.tar.gz > /tmp/zstd.tar.gz \
 && tar -xvf /tmp/zstd.tar.gz -C /tmp \
 && make -C /tmp/zstd-1.0.0/programs install \
 && rm -rf /tmp/zstd-1.0.0/ /tmp/zstd.tar.gz \
    ;

ENV GOPATH      /go
ENV PATH        /usr/local/go/bin:$GOPATH/bin:$PATH
ENV APP_PATH    github.com/taskcluster/taskcluster-worker
ENV SHELL       bash

RUN mkdir -p /go/src/$APP_PATH
RUN ln -s /go/src/$APP_PATH /src

RUN go get github.com/smartystreets/goconvey

WORKDIR /go/src/$APP_PATH
