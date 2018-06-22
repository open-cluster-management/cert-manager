FROM alpine:3.6

ARG VCS_REF
ARG VCS_URL
ARG IMAGE_NAME
ARG IMAGE_DESCRIPTION
ARG GOARCH

RUN apk add --no-cache ca-certificates && \
  addgroup -S certmanager && adduser -S -G certmanager certmanager

ADD icp-cert-manager-controller_linux_$GOARCH /usr/bin/cert-manager

USER certmanager

ENTRYPOINT ["/usr/bin/cert-manager"]

# http://label-schema.org/rc1/
LABEL org.label-schema.vendor="IBM" \
      org.label-schema.name="$IMAGE_NAME" \
      org.label-schema.description="$IMAGE_DESCRIPTION" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.vcs-url=$VCS_URL \
      org.label-schema.license="Licensed Materials - Property of IBM" \
      org.label-schema.schema-version="1.0"