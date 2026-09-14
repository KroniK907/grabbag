# Minimal base for agent-config cloud bootstrap.
# Extend for project toolchain (Go version, Node, etc.).
FROM ubuntu:24.04

RUN apt-get update \
    && apt-get install -y --no-install-recommends git curl jq ca-certificates sudo \
    && rm -rf /var/lib/apt/lists/*
