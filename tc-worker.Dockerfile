FROM ubuntu:17.04
MAINTAINER Jonas Finnemann Jensen <jopsen@gmail.com>

# Install runtime dependencies
RUN apt-get update -y \
 && apt-get upgrade -y \
 && apt-get install -y \
    qemu-system-x86 qemu-utils dnsmasq-base iptables iproute2 openvpn \
    netcat-openbsd p7zip-full genisoimage \
 && apt-get clean -y \
 && rm -rf /var/lib/apt/lists/

# Install zstd 1.1.4
RUN apt-get update -y \
 && apt-get install -y curl build-essential \
 && curl -L https://github.com/facebook/zstd/archive/v1.3.0.tar.gz > /tmp/zstd.tar.gz \
 && tar -xvf /tmp/zstd.tar.gz -C /tmp \
 && make -C /tmp/zstd-1.3.0/programs install \
 && rm -rf /tmp/zstd-1.3.0/ /tmp/zstd.tar.gz \
 && apt-get purge -y curl build-essential \
 && apt-get auto-remove -y \
 && apt-get clean -y \
 && rm -rf /var/lib/apt/lists/

# Install taskcluster-worker
RUN           mkdir -p /usr/local/bin/
COPY          taskcluster-worker /usr/local/bin/taskcluster-worker

# Install configuration file
COPY          examples/packet-config.yml /etc/taskcluster-worker.yml

# Ensure that a data volume is present at /mnt
VOLUME        /mnt

# Expose PORT for interactive features
EXPOSE        443
ENV           PORT    443

# Set working directory and entrypoint
WORKDIR       /root
ENTRYPOINT    ["/usr/local/bin/taskcluster-worker"]
CMD           ["work", "/etc/taskcluster-worker.yml"]
