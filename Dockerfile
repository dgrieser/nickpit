# ---- Go build ------------------------------------------------------------
FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /nickpit ./cmd/nickpit

# ---- Stage git + its shared libs (bookworm, matches base-debian12) -------
FROM debian:12-slim AS gitpkg
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
# Collect git, its core helpers (git-remote-https is needed for HTTPS clone),
# and every transitive .so into /bundle, preserving absolute paths (cp --parents)
# so arch-specific lib dirs (x86_64/aarch64/arm-linux-gnueabihf) land correctly
# per build platform under buildx/QEMU.
RUN set -eu; mkdir -p /bundle; \
    cp --parents /usr/bin/git /bundle; \
    cp --parents -r /usr/lib/git-core /bundle; \
    for f in /usr/bin/git $(find /usr/lib/git-core -type f); do \
        ldd "$f" 2>/dev/null | awk '/=>/{print $3} /ld-linux/{print $1}'; \
    done | sort -u | while read -r lib; do \
        [ -e "$lib" ] && cp --parents "$lib" /bundle || true; \
    done; \
    cp --parents -r /usr/share/git-core /bundle 2>/dev/null || true

# ---- Runtime: distroless base, rootless (nonroot = UID 65532) ------------
# base-debian12 (not static) is required because the bundled git needs glibc and
# shared libraries. It provides ca-certificates, /etc/passwd with a nonroot user,
# HOME=/home/nonroot, tzdata, and /tmp at mode 1777 (writable by any UID).
FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=gitpkg /bundle/ /
COPY --from=builder /nickpit /usr/local/bin/nickpit
# git CLI is back, so a host-mounted repo owned by a different UID would trip
# git's "dubious ownership" guard. Inject safe.directory=* via env (git >= 2.31),
# HOME-/UID-independent; the Go code does not scrub env, so every git call sees it.
ENV GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0=safe.directory \
    GIT_CONFIG_VALUE_0=* \
    GIT_EXEC_PATH=/usr/lib/git-core
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/nickpit"]
