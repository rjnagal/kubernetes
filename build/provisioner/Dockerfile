FROM progrium/busybox
MAINTAINER vmarmol@google.com

ADD provisioner /usr/bin/provisioner

CMD ["/usr/bin/provisioner", "-logtostderr", "-v=2", "-listen_ip=localhost"]


