FROM ubuntu:18.04
MAINTAINER Jonas Finnemann Jensen <jopsen@gmail.com>

# Install dependencies for setting up
RUN apt-get update -y \
 && apt-get upgrade -y \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    git curl screen nano build-essential

# Install golang 1.10
RUN curl -L https://storage.googleapis.com/golang/go1.10.linux-amd64.tar.gz > /tmp/go.tar.gz \
 && tar -xvf /tmp/go.tar.gz -C /usr/local \
 && rm /tmp/go.tar.gz

RUN mkdir -p /go
ENV GOPATH      /go
ENV PATH        /usr/local/go/bin:$GOPATH/bin:$PATH

# Install govendor
RUN go get github.com/kardianos/govendor

# Install dependencies for QEMU engine tests
RUN apt-get update -y \
 && apt-get upgrade -y \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    qemu-system-x86 qemu-utils dnsmasq-base iptables iproute2 netcat-openbsd \
    genisoimage openvpn awscli jq p7zip-full

# Install zstd 1.1.4
RUN curl -L https://github.com/facebook/zstd/archive/v1.1.4.tar.gz > /tmp/zstd.tar.gz \
 && tar -xvf /tmp/zstd.tar.gz -C /tmp \
 && make -j -C /tmp/zstd-1.1.4/programs install \
 && rm -rf /tmp/zstd-1.1.4/ /tmp/zstd.tar.gz

# Install docker
RUN apt-get update -y \
 && apt-get install -y apt-transport-https ca-certificates \
 && curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add - \
 && echo 'deb [arch=amd64] https://download.docker.com/linux/ubuntu zesty stable' > /etc/apt/sources.list.d/docker.list \
 && apt-get update -y \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce btrfs-progs e2fsprogs iptables xfsprogs xz-utils

# Setup environment for running go test
ENV APP_PATH    github.com/taskcluster/taskcluster-worker
ENV SHELL       bash

RUN mkdir -p /go/src/$APP_PATH
WORKDIR /go/src/$APP_PATH

# Use volume for docker layers
VOLUME /var/lib/docker

# Inject command wrapper script to launch dockerd
RUN echo '#!/bin/bash\n\
dockerd -s vfs >/var/log/docker.log 2>&1 &\n\
while [ ! -S /var/run/docker.sock ]; do sleep 0.1; done\n\
"$@"\n\
RETVAL=$?\n\
kill %1\n\
exit "$RETVAL"' > /usr/local/bin/with-dockerd.sh \
  && chmod +x /usr/local/bin/with-dockerd.sh

#trap "kill $!; wait $!" EXIT\n\
ENTRYPOINT ["/usr/local/bin/with-dockerd.sh"]
