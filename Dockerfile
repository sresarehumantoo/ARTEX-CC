# syntax=docker/dockerfile:1
#
# Multi-stage build for ARTEX. Frontend is built in a Node stage, then
# embedded into the Go binary via go:embed.

# -------------------------------------------------------------- frontend ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# --------------------------------------------------------------- builder ---
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist /src/web/dist
ARG TARGETOS
ARG TARGETARCH
ARG ARTEX_VERSION=dev
RUN CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-amd64}" \
    go build -trimpath \
      -ldflags="-s -w -X main.version=${ARTEX_VERSION}" \
      -o /out/artex-cc ./cmd/artex-cc

# --------------------------------------------------------------- runtime ---
FROM alpine:3.21
LABEL org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.source="https://github.com/SREsAreHumanToo/ARTEX-CC"
RUN apk add --no-cache ca-certificates nodejs npm \
    && npm install -g @anthropic-ai/claude-code@latest \
    && npm cache clean --force
# --------------------------------------------------------- pentest tools ---
# nmap, ldapsearch, smbclient, rpcclient, curl, wget, john, netcat, socat,
# ssh, dig, impacket, bloodhound-python, evil-winrm, nuclei, ffuf.
# Set --build-arg PENTEST_TOOLS=0 to skip this layer.
ARG PENTEST_TOOLS=1
ARG NUCLEI_VERSION=3.3.7
ARG FFUF_VERSION=2.1.0
RUN set -eux; \
    if [ "${PENTEST_TOOLS}" != "1" ]; then exit 0; fi; \
    apk add --no-cache --virtual .pentest-build-deps \
        gcc g++ musl-dev make \
        python3-dev ruby-dev \
        libffi-dev openssl-dev \
        krb5-dev linux-headers \
        readline-dev \
        unzip \
    && apk add --no-cache \
        python3 py3-pip py3-cryptography \
        ruby \
        nmap nmap-scripts \
        openldap-clients \
        samba-client \
        curl wget \
        john \
        netcat-openbsd socat \
        openssh-client \
        bind-tools \
        libffi krb5-libs \
    && pip install --break-system-packages --no-cache-dir \
        impacket \
        bloodhound \
    && gem install --no-document evil-winrm \
    && ARCH="$(apk --print-arch)" \
    && case "$ARCH" in x86_64) DLARCH=amd64;; aarch64) DLARCH=arm64;; *) DLARCH="$ARCH";; esac \
    && wget -qO /tmp/nuclei.zip \
         "https://github.com/projectdiscovery/nuclei/releases/download/v${NUCLEI_VERSION}/nuclei_${NUCLEI_VERSION}_linux_${DLARCH}.zip" \
    && unzip -oq /tmp/nuclei.zip nuclei -d /usr/local/bin \
    && chmod +x /usr/local/bin/nuclei \
    && wget -qO- \
         "https://github.com/ffuf/ffuf/releases/download/v${FFUF_VERSION}/ffuf_${FFUF_VERSION}_linux_${DLARCH}.tar.gz" \
       | tar -xzf - -C /usr/local/bin ffuf \
    && chmod +x /usr/local/bin/ffuf \
    && apk del .pentest-build-deps \
    && rm -rf /root/.cache /root/.cargo /tmp/*

WORKDIR /app
COPY --from=build /out/artex-cc /app/artex-cc
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
COPY LICENSE NOTICE /app/
RUN chmod +x /app/artex-cc /app/docker-entrypoint.sh
VOLUME ["/app/data"]
EXPOSE 9400 8788
ENTRYPOINT ["/app/docker-entrypoint.sh"]
