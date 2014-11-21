FROM progrium/busybox
MAINTAINER vishnuk@google.com

ADD scaler /usr/bin/scaler

CMD ["/usr/bin/scaler", "-logtostderr", "-v=2", "-cluster_scaling_policy=minute"]

