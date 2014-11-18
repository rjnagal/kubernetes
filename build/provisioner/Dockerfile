FROM progrium/busybox
MAINTAINER vmarmol@google.com

ADD provisioner /usr/bin/provisioner
RUN chmod +x /usr/bin/provisioner

CMD ["/usr/bin/provisioner", "-logtostderr", "-v=2"]


