FROM progrium/busybox
MAINTAINER vmarmol@google.com

# Grab cadvisor from the staging directory.
ADD provisioner /usr/bin/provisioner
RUN chmod +x /usr/bin/provisioner

CMD ["/usr/bin/provisioner", "-logtostderr"]


