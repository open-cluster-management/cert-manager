FROM alpine:3.6

ARG VCS_REF
ARG VCS_URL
ARG IMAGE_NAME
ARG IMAGE_DESCRIPTION

RUN apk add --no-cache ca-certificates && \
    adduser -S -u 100 acmesolver

ADD cert-manager-acmesolver_linux_amd64 /usr/bin/acmesolver

USER 100
ENTRYPOINT ["/usr/bin/acmesolver"]


# http://label-schema.org/rc1/
LABEL org.label-schema.vendor="IBM" \
      org.label-schema.name="$IMAGE_NAME" \
      org.label-schema.description="$IMAGE_DESCRIPTION" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.vcs-url=$VCS_URL \
      org.label-schema.license="Licensed Materials - Property of IBM" \
      org.label-schema.schema-version="1.0"