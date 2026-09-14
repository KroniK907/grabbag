# Base image for the hackbox Cloud Agent environment.
# System packages + Go toolchain live here (stable, slow-changing).
# Repository dependency work lives in environment.json "install".
FROM ubuntu:24.04

RUN apt-get update \
    && apt-get install -y --no-install-recommends git curl jq ca-certificates sudo \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain. hackbox is a single Go module (see docs/agent/CODEBASE.md).
ARG GO_VERSION=1.27.1
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
