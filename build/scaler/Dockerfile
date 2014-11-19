FROM progrium/busybox
MAINTAINER vishnuk@google.com

ADD scaler /usr/bin/scaler
RUN chmod +x /usr/bin/scaler

CMD ["/usr/bin/scaler", "-logtostderr", "-v=2"]

