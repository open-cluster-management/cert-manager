FROM registry.access.redhat.com/ubi8/ubi-minimal:8.1-398

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
    adduser -r -g webhook -u 10001 webhook && \
    microdnf clean all

ADD cert-manager-webhook_linux_$GOARCH /usr/bin/webhook
RUN chmod a+x /usr/bin/webhook && mkdir licenses

COPY LICENSE licenses

WORKDIR /home/webhook
USER 10001

RUN mkdir tmp && chmod 777 tmp

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
