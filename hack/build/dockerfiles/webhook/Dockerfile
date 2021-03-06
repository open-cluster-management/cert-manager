# Copyright Contributors to the Open Cluster Management project

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG VCS_REF
ARG VCS_URL
ARG IMAGE_NAME
ARG IMAGE_DESCRIPTION
ARG SUMMARY
ARG GOARCH
ARG HOME

RUN microdnf update && \
    microdnf install shadow-utils ca-certificates procps && \
    groupadd -r webhook && \
    adduser -rm -g webhook -l -u 100101 webhook && \
    microdnf clean all

RUN mkdir licenses

COPY LICENSE licenses

USER 100101

WORKDIR /tmp
RUN mkdir tmp
ADD cert-manager-webhook_linux_$GOARCH /usr/bin/webhook

ENTRYPOINT ["/usr/bin/webhook"]

# http://label-schema.org/rc1/
LABEL org.label-schema.vendor="Red Hat" \
      org.label-schema.name="$IMAGE_NAME" \
      org.label-schema.description="$IMAGE_DESCRIPTION" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.vcs-url=$VCS_URL \
      org.label-schema.license="Red Hat Advanced Cluster Management for Kubernetes EULA" \
      org.label-schema.schema-version="1.0"

LABEL name="$IMAGE_NAME"
LABEL vendor="IBM"
LABEL version="1.0"
LABEL release="$VCS_REF"
LABEL summary="$SUMMARY"
LABEL description="$IMAGE_DESCRIPTION"
