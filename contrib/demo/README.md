demo
====

These instructions will take you through setting up vagrant and libvirt and using these tools to 
setup and execute a geard demo.

1. [Host setup](#host-setup)
1. [Demo setup](#demo-setup)

Host setup
-------------

1.  Install Vagrant 1.6.2

        $ wget https://dl.bintray.com/mitchellh/vagrant/vagrant_1.6.2_x86_64.rpm
        $ yum -y --nogpgcheck localinstall vagrant_1.6.2_x86_64.rpm
        $ vagrant version
        Installed Version: 1.6.2
        Latest Version: 1.6.2

2.  Setup libvirt 

        $ sudo yum install @virtualization
        $ wget http://file.rdu.redhat.com/~decarr/dockercon/libvirt-setup.sh
        $ chmod u+x libvirt-setup.sh
        $ sudo ./libvirt-setup.sh
        $ sudo systemctl start libvirtd.service
        $ sudo systemctl enable libvirtd.service

3. Setup vagrant with libvirt

        $ sudo yum install -y wget tree vim screen mtr nmap telnet tar git
        $ curl -sSL https://get.rvm.io | bash -s stable
        $ sudo yum install -y libvirt-devel libxslt-devel libxml2-devel
        $ gem install nokogiri -v '1.5.11'
        $ vagrant plugin install --plugin-version 0.0.16 vagrant-libvirt

4.  Setup Vagrant Box and review Vagrantfile

        vagrant box add --name=atomic --provider=libvirt http://rcm-img06.build.bos.redhat.com/images/releases/snaps/20140522.0/vagrant/rhel-atomic-host-vagrant.box

    Clone pmorie's fork and check out the `multi-demo` branch:

        $ git clone git://github.com/pmorie/geard
        $ cd geard
        $ git checkout demo-multi
        $ cd contrib/demo
        $ cat Vagrantfile

5.  Pull and start local docker registry

        $ docker pull pmorie/geard-demo-registry
        $ docker run -p 5000:5000 pmorie/geard-demo-registry

6.  Start VMs

        $ vagrant up --provider=libvirt
        Bringing machine 'default' up with 'libvirt' provider...
        ==> default: HandleBoxUrl middleware is deprecated. Use HandleBox instead.
        ==> default: This is a bug with the provider. Please contact the creator
        ==> default: of the provider you use to fix this.
        ==> default: Creating image (snapshot of base box volume).
        ==> default: Creating domain with the following settings...
        ==> default:  -- Name:          dockercon_489e2b0ea1cb9397b6db6b1bd3a89ad8
        ==> default:  -- Domain type:   kvm
        ==> default:  -- Cpus:          4
        ==> default:  -- Memory:        4096M
        ==> default:  -- Base box:      atomic
        ==> default:  -- Storage pool:  default
        ==> default:  -- Image:         /var/lib/libvirt/images/dockercon_489e2b0ea1cb9397b6db6b1bd3a89ad8.img
        ==> default:  -- Volume Cache:  default
        ==> default: Creating shared folders metadata...
        ==> default: Starting domain.
        ==> default: Waiting for domain to get an IP address...
        ==> default: Waiting for SSH to become available...

    If you run into issues, try the following:

        $ rm -fr ./vagrant
        $ vagrant plugin list

    If you have other third-party plug-ins installed, try to remove them.  In particular, we found
    errors when running the following plug-ins with vagrant-libvirt:

    1.  vagrant-aws
    2.  vagrant-openshift

    Be patient. This will bring up two vm instances: `vm1` and `vm2`.  The provisioner on the 
    initial vagrant up will fetch all required docker images to support the  demo from the local
    registry, and git clone required content.

7.  SSH access

    Validate that you can reach the VMs with ssh:

        $ vagrant ssh vm1
        $ docker images
        $ cd geard
        $ git branch
        * demo-multi
          master

        $ vagrant ssh vm2
        $ docker images
        * demo-multi
          master

    You can now run rpm-ostree commands, etc.  You can also see the vm running by viewing in 
    virt-manager:

        $ virt-manager

    You should see be able to see both instances running.

8.  OSTree Upgrade

    Next, we need to update to the latest packages.  On both VMs, run the following:

        $ rpm-ostree upgrade

    Then reboot the VMs:

        $ vagrant halt
        $ vagrant up --provider=libvirt && vagrant provision

    Note: The `vagrant-libvirt` provider does not support the `vagrant up --provision` flag.  The 
    provision step is required to work around an issue where the private network interface is not
    brought up on boot by vagrant controller for each VM to communicate.

Demo Setup
----------

Once the host is set up, you can deploy the demo containers:

        $ cd geard
        $ contrib/demo/setup-multi.sh

This will install the following containers:

        parks-backend-{1,2,3}
        parks-db-1
        parks-lb-1

The setup script will leave `parks-backend-{2,3}` stopped, to be started for scale-up during 
the demo.  Once the script has run, you should be able to hit the demo from your host in a 
browser at: `http://localhost:14000`
