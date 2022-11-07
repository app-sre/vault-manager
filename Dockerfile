FROM registry.access.redhat.com/ubi8/go-toolset:1.17 as builder
COPY . .
RUN make gobuild

FROM registry.access.redhat.com/ubi8-minimal:8.2
RUN microdnf update -y && microdnf install -y ca-certificates && rm -rf /var/cache/yum
COPY --from=builder /opt/app-root/src/vault-manager /
COPY query.graphql liveness.sh /
ENTRYPOINT ["/vault-manager"]
