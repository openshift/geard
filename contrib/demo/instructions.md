These instructions will take you from a clean VM* to a working demo app.  In future iterations these instructions will handle creating the VMs for demo hosts.

Libvirt setup
-------------

1. Install Vagrant 1.6.2

        $ wget https://dl.bintray.com/mitchellh/vagrant/vagrant_1.6.2_x86_64.rpm
        $ yum -y --nogpgcheck localinstall vagrant_1.6.2_x86_64.rpm
        $ vagrant version
        Installed Version: 1.6.2
        Latest Version: 1.6.2

1. Setup libvirt 

        $ sudo yum install @virtualization
        $ wget http://file.rdu.redhat.com/~decarr/dockercon/libvirt-setup.sh
        $ chmod u+x libvirt-setup.sh
        $ sudo ./libvirt-setup.sh
        $ sudo systemctl start libvirtd.service
        $ sudo systemctl enable libvirtd.service

1. Setup vagrant with libvirt

        $ sudo yum install -y wget tree vim screen mtr nmap telnet tar git
        $ curl -sSL https://get.rvm.io | bash -s stable
        $ sudo yum install -y libvirt-devel libxslt-devel libxml2-devel
        $ gem install nokogiri -v '1.5.11'
        $ vagrant plugin install --plugin-version 0.0.16 vagrant-libvirt

1.  Setup Vagrant Box and review Vagrantfile

        vagrant box add --name=atomic --provider=libvirt http://rcm-img06.build.bos.redhat.com/images/releases/snaps/20140522.0/vagrant/rhel-atomic-host-vagrant.box

Assume you have some location to hold Vagrantfile, i.e. ~/dockercon

    $ git clone git://github.com/pmorie/geard
    $ cd geard
    $ git checkout demo-multi
    $ cd contrib/demo
    $ cat Vagrantfile

1.  Pull and start local docker registry

        $ docker pull pmorie/geard-demo-registry
        $ docker run -p 5000:5000 pmorie/geard-demo-registry

1.  Start VMs

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

If you have other third-party plug-ins installed, try to remove them.  In particular, we found erors when running the following plug-ins with vagrant-libvirt:

    1. vagrant-aws
    2. vagrant-openshift

Be patient.

This will bring up two vm instances: vm1 and vm2

The provisioner on the initial vagrant up will fetch all required docker images to support the demo, and git clone required content.

SSH

Validate all is good.

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

You can now run rpm-ostree commands, etc.

You can also see the vm running by viewing in virt-manager:

$ virt-manager

You should see both instances running.

Demo Setup
----------

These instructions assume that you already have a VM with geard and docker.  Before you get started:

1.  Make sure that the docker and geard daemons are running.
1.  Make sure that port `14000` on the VM is mapped to `14000` on your host machine.
1.  Make sure that firewalld is stopped if using fedora as the OS for the VM:

    sudo systemctl stop firewalld.service

1.  Next, run the demo setup script:
    
    cd geard
    contrib/demo/setup-multi.sh

This will install the following containers:

    parks-backend-{1,2,3}
    parks-db-1
    parks-lb-1

The setup script will leave `parks-backend-{2,3}` stopped, to be started for scale-up during the demo.  Once the script has run, you should be able to hit the demo from your host in a browser at:

    http://localhost:14000
