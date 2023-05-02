# -*- mode: ruby -*-
# vi: set ft=ruby :

# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.
Vagrant.configure("2") do |config|
  # The most common configuration options are documented and commented below.
  # For a complete reference, please see the online documentation at
  # https://docs.vagrantup.com.

  # Every Vagrant development environment requires a box. You can search for
  # boxes at https://vagrantcloud.com/search.
  config.vm.box = "fedora/38-cloud-base"

  # Disable automatic box update checking. If you disable this, then
  # boxes will only be checked for updates when the user runs
  # `vagrant box outdated`. This is not recommended.
  # config.vm.box_check_update = false

  # Create a forwarded port mapping which allows access to a specific port
  # within the machine from a port on the host machine. In the example below,
  # accessing "localhost:8080" will access port 80 on the guest machine.
  # NOTE: This will enable public access to the opened port
  # config.vm.network "forwarded_port", guest: 80, host: 8080
  config.vm.network "forwarded_port", guest: 8000, host: 8000 # tempest
  config.vm.network "forwarded_port", guest: 6090, host: 6090 # sandstorm

  # Create a forwarded port mapping which allows access to a specific port
  # within the machine from a port on the host machine and only allow access
  # via 127.0.0.1 to disable public access
  # config.vm.network "forwarded_port", guest: 80, host: 8080, host_ip: "127.0.0.1"

  # Create a private network, which allows host-only access to the machine
  # using a specific IP.
  # config.vm.network "private_network", ip: "192.168.33.10"

  # Create a public network, which generally matched to bridged network.
  # Bridged networks make the machine appear as another physical device on
  # your network.
  # config.vm.network "public_network"

  # Share an additional folder to the guest VM. The first argument is
  # the path on the host to the actual folder. The second argument is
  # the path on the guest to mount the folder. And the optional third
  # argument is a set of non-required options.
  # config.vm.synced_folder "../data", "/vagrant_data"

  # Provider-specific configuration so you can fine-tune various
  # backing providers for Vagrant. These expose provider-specific options.
  # Example for VirtualBox:
  #
  # config.vm.provider "virtualbox" do |vb|
  #   # Display the VirtualBox GUI when booting the machine
  #   vb.gui = true
  #
  #   # Customize the amount of memory on the VM:
  #   vb.memory = "1024"
  # end
  #
  # View the documentation for the provider you are using for more
  # information on available options.
  config.vm.provider "virtualbox" do |vb|
    vb.cpus = `nproc`.to_i
    vb.memory = 2048
  end

  # Enable provisioning with a shell script. Additional provisioners such as
  # Ansible, Chef, Docker, Puppet and Salt are also available. Please see the
  # documentation for more information about their specific syntax and use.
  config.vm.provision "shell", inline: <<-SHELL
    set -exuo pipefail

    dnf install -y \
      go \
      tinygo \
      binaryen \
      capnproto \
      flex \
      bison

    go install capnproto.org/go/capnp/v3/capnpc-go@latest

    # Install the BPF assembler:
    curl https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.3.1.tar.xz > linux.tar.xz
    tar -xvf linux.tar.xz
    cd linux-*/tools/bpf
    make bpf_asm
    install -Dm755 -t /usr/local/bin/ bpf_asm

    # Install sandstorm:
    curl https://install.sandstorm.io/ 2>&1 > ~/install-sandstorm.sh
    bash ~/install-sandstorm.sh -d -e -p 6090
    # Make the vagrant user part of the sandstorm group so that commands like
    # `spk dev` work.
    usermod -a -G 'sandstorm' 'vagrant'
    # Bind to all addresses, so the vagrant port-forward works.
    sudo sed --in-place='' \
            --expression='s/^BIND_IP=.*/BIND_IP=0.0.0.0/' \
            /opt/sandstorm/sandstorm.conf
    sudo systemctl restart sandstorm
  SHELL
end
