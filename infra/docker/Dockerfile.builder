# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM debian:13-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends bash ca-certificates curl git tar xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl --fail --silent --show-error --location \
      --retry 5 --retry-all-errors --retry-delay 3 \
      https://mise.run \
      --output /tmp/mise-install.sh \
    && MISE_VERSION=v2026.10.17 MISE_INSTALL_PATH=/usr/local/bin/mise sh /tmp/mise-install.sh \
    && rm /tmp/mise-install.sh

WORKDIR /workspace

ENV MISE_TRUSTED_CONFIG_PATHS=/workspace \
    MISE_AUTO_INSTALL=0 \
    GOPATH=/go \
    PATH="/root/.local/share/mise/shims:${PATH}"

COPY mise.toml mise.lock ./

RUN mkdir -p apps/api apps/web packages/node/configs \
    && mise install --locked node go pnpm

LABEL org.opencontainers.image.title="HeyBlog builder"
