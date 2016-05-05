Vagrant.configure(2) do |config|
  config.vm.box = "debian/jessie64"

  # Disable guest tools, for those who have the vbguest plugin
  config.vbguest.no_install = true

  # Disable the default folder synchronization
  # keep it sync'ed by running: vagrant rsync-auto
  config.vm.synced_folder ".", "/vagrant", type: "rsync", disabled: true

  # Synchronize taskcluster-worker folder into the GOPATH
  config.vm.synced_folder ".", "/go/src/github.com/taskcluster/taskcluster-worker/", type: "rsync",
    rsync__args: ["--verbose", "--archive", "--delete", "-z", "--copy-links", "--sparse"],
    rsync__exclude: [".vagrant/", ".git/"],
    rsync__auto: true

  # Forward ports for VNC and 8080 (where we run the live log, etc)
  config.vm.network "forwarded_port", guest: 5900, host: 5900
  config.vm.network "forwarded_port", guest: 8080, host: 8080

  # Modify VM to consume less resources, otherwise virtualbox easily gets hot
  # when running QEMU without KVM acceleration
  config.vm.provider "virtualbox" do |vb|
    vb.memory = "1024"
    vb.customize ["modifyvm", :id, "--cpuexecutioncap", "50"]
  end

  # Install QEMU, dnsmasq, go and configure a nice environment
  config.vm.provision "shell", inline: <<-SHELL
    sudo apt-get update -y
    sudo apt-get install -y qemu dnsmasq-base build-essential git
    curl -s https://storage.googleapis.com/golang/go1.6.2.linux-amd64.tar.gz > /tmp/go.tar.gz
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    sudo bash -c "echo 'export PATH=\$PATH:/usr/local/go/bin' >> /etc/profile"
    sudo bash -c "echo 'export GOPATH=/go' >> /etc/profile"
    echo "PS1='[vagrant > \\W]$'" >> /home/vagrant/.bashrc
    echo "source /home/vagrant/.bashrc" >> /home/vagrant/.bash_profile
    echo "cd /go/src/github.com/taskcluster/taskcluster-worker/" >> /home/vagrant/.bash_profile
    echo "sudo bash --login" >> /home/vagrant/.bash_profile
    echo "PS1='[vagrant > \\W]#'" >> /root/.bashrc
    sudo mkdir -p /go/src/github.com/taskcluster/taskcluster-worker/
    sudo chown -R vagrant:vagrant /go
  SHELL
end
