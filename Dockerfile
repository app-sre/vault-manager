FROM alpine:3.7
RUN apk add --no-cache ca-certificates
RUN adduser -D -g '' vault-manager
USER vault-manager
COPY vault-manager query.graphql /
ENTRYPOINT ["/vault-manager"]
