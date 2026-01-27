# ============================================================================
# Stage 1: Builder - Compile the vault-manager binary
# ============================================================================
FROM registry.access.redhat.com/ubi9/go-toolset:1.24.6 AS builder
COPY --chown=1001:0 . .
RUN make gobuild

# ============================================================================
# Stage 2: Production - Minimal runtime image (DEFAULT TARGET)
# ============================================================================
FROM registry.access.redhat.com/ubi9-minimal:9.5 AS production
RUN microdnf update -y && \
    microdnf install -y ca-certificates && \
    rm -rf /var/cache/yum

COPY --from=builder /opt/app-root/src/vault-manager /
COPY query.graphql /
ENTRYPOINT ["/vault-manager"]
LABEL konflux.additional-tags="1.0.0"

# ============================================================================
# Stage 3: Test - Full test environment with BATS and dependencies
# ============================================================================
FROM registry.access.redhat.com/ubi9/ubi:9.5 AS test

# Add Tini init system
ENV TINI_VERSION=v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini
RUN chmod +x /tini

# Version pinning for reproducible builds
ENV BATS_VERSION="v1.11.1"
ENV VAULT_VERSION="1.19.5"

# Install test dependencies
RUN yum install -y \
    ca-certificates \
    git \
    jq \
    podman \
    python3-pip \
    unzip \
    wget && \
    yum clean all && \
    update-ca-trust extract

# Install BATS testing framework
RUN git clone https://github.com/bats-core/bats-core.git && \
    git --git-dir=bats-core/.git checkout -b $BATS_VERSION $BATS_VERSION >/dev/null && \
    bats-core/install.sh /usr/local

# Install Vault CLI for integration testing
RUN wget -q https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip && \
    unzip vault_${VAULT_VERSION}_linux_amd64.zip && \
    mv vault /usr/bin && \
    rm vault_${VAULT_VERSION}_linux_amd64.zip

# Install podman-compose
RUN pip3 install podman-compose

# Copy test suite
COPY tests/ /tests/

# Copy vault-manager binary from builder stage
COPY --from=builder /opt/app-root/src/vault-manager /bin/

WORKDIR /tests

ENTRYPOINT ["/tini", "--"]
CMD ["sh", "-c", "while true; do sleep 10; done"]
