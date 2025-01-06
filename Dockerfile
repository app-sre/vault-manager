FROM registry.access.redhat.com/ubi8/go-toolset:1.21 AS builder
COPY . .
RUN make gobuild

FROM registry.access.redhat.com/ubi9-minimal:9.4
RUN microdnf update -y && \
    microdnf install -y ca-certificates && \
    rm -rf /var/cache/yum

COPY --from=builder /opt/app-root/src/vault-manager /
COPY query.graphql /
ENTRYPOINT ["/vault-manager"]
