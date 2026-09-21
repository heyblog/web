# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

FROM debian:13-slim@sha256:d7e12182ce18b85b93007c1dedf31f2d29e01ccf3182cc4017c709b6259bc132

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends bash ca-certificates curl git tar xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl --fail --silent --show-error --location \
      --retry 5 --retry-all-errors --retry-delay 3 \
      https://mise.run \
      --output /tmp/mise-install.sh \
    && MISE_VERSION=v2026.9.12 MISE_INSTALL_PATH=/usr/local/bin/mise sh /tmp/mise-install.sh \
    && rm /tmp/mise-install.sh

WORKDIR /workspace

ENV MISE_TRUSTED_CONFIG_PATHS=/workspace \
    MISE_AUTO_INSTALL=0 \
    PATH="/root/.local/share/mise/shims:${PATH}"

COPY mise.toml mise.lock ./

RUN mkdir -p apps/api apps/web packages/node/configs \
    && mise install --locked node go pnpm

LABEL org.opencontainers.image.title="HeyBlog builder"
