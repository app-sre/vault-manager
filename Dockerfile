FROM registry.access.redhat.com/ubi9/go-toolset:1.25.5-1769430014 AS builder
COPY . .
RUN make gobuild
LABEL konflux.additional-tags="1.0.0"

FROM registry.access.redhat.com/ubi9-minimal:9.5
RUN microdnf update -y && \
    microdnf install -y ca-certificates && \
    rm -rf /var/cache/yum

COPY --from=builder /opt/app-root/src/vault-manager /
COPY query.graphql /
ENTRYPOINT ["/vault-manager"]
