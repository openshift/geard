echo "Building and Loading Policy"
set -x
if [ ! -f /usr/share/selinux/devel/Makefile ] ; then
	yum install -y selinux-policy-devel
fi
make -f /usr/share/selinux/devel/Makefile
/usr/sbin/semodule -i geard.pp
semanage fcontext -d -e /home '/var/lib/containers/home/([^/]*)/([^/]*)'
semanage fcontext -a -e /home '/var/lib/containers/home/([^/]*)/([^/]*)'
