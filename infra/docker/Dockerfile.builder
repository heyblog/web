# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

FROM debian:13-slim@sha256:d7e12182ce18b85b93007c1dedf31f2d29e01ccf3182cc4017c709b6259bc132

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ENV NVM_DIR=/usr/local/nvm

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates curl git jq tar xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p "${NVM_DIR}" \
    && curl --fail --silent --show-error --location \
      --retry 5 --retry-all-errors --retry-delay 3 \
      "https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh" \
      --output /tmp/nvm-install.sh \
    && PROFILE=/dev/null NVM_DIR="${NVM_DIR}" bash /tmp/nvm-install.sh \
    && source "${NVM_DIR}/nvm.sh" \
    && nvm install 24.19.0 \
    && nvm alias default 24.19.0 \
    && nvm cache clear \
    && rm /tmp/nvm-install.sh

ENV PATH="${NVM_DIR}/versions/node/v24.19.0/bin:${PATH}"

RUN corepack enable

RUN go_filename="go1.26.6.linux-amd64.tar.gz" \
    && go_sha256="$(curl --fail --silent --show-error --location \
      --retry 5 --retry-all-errors --retry-delay 3 \
      'https://go.dev/dl/?mode=json&include=all' \
      | jq -r --arg filename "${go_filename}" \
        '.[].files[] | select(.filename == $filename) | .sha256')" \
    && test -n "${go_sha256}" \
    && test "${go_sha256}" != null \
    && curl --fail --silent --show-error --location \
      --retry 5 --retry-all-errors --retry-delay 3 \
      "https://go.dev/dl/${go_filename}" \
      --output /tmp/go.tar.gz \
    && echo "${go_sha256}  /tmp/go.tar.gz" | sha256sum --check --status \
    && tar --extract --file=/tmp/go.tar.gz --directory=/usr/local \
    && rm /tmp/go.tar.gz

ENV GOPATH=/go \
    PATH="/usr/local/nvm/versions/node/v24.19.0/bin:/usr/local/go/bin:/go/bin:${PATH}"

RUN curl --fail --silent --show-error --location \
      --retry 5 --retry-all-errors --retry-delay 3 \
      https://taskfile.dev/install.sh \
      --output /tmp/task-install.sh \
    && bash /tmp/task-install.sh -d -b /usr/local/bin v3.52.0 \
    && rm /tmp/task-install.sh \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

LABEL org.opencontainers.image.title="HeyBlog builder"
