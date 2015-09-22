# DOCKER-VERSION 1.8.0
# VERSION 0.0.1

FROM alpine
MAINTAINER Seán C McCord "ulexus@gmail.com"

ADD bin/ccproxy /ccproxy

ENTRYPOINT []
CMD ["/ccproxy"]
