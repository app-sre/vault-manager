FROM registry.access.redhat.com/ubi8/go-toolset:latest as builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo ./cmd/vault-manager

FROM registry.access.redhat.com/ubi8-minimal:8.2
RUN microdnf update -y && microdnf install -y ca-certificates && rm -rf /var/cache/yum
COPY --from=builder /opt/app-root/src/vault-manager /
COPY query.graphql /
ENTRYPOINT ["/vault-manager"]
