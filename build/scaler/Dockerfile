FROM progrium/busybox
MAINTAINER vishnuk@google.com

# Grab cadvisor from the staging directory.
ADD scaler /usr/bin/scaler
RUN chmod +x /usr/bin/scaler

CMD ["/usr/bin/scaler", "-logtostderr"]

