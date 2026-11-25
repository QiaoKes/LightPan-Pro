# syntax=docker/dockerfile:1

ARG DEBIAN_IMAGE=debian:bookworm-slim

FROM node:22-bookworm-slim AS web

WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run type-check && npm run build


FROM golang:1.26.6-bookworm AS build

WORKDIR /src

# Keep the builder aligned with go.mod without downloading another toolchain.
ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOPROXY=https://proxy.golang.org,direct
ARG BUILD_TAGS=fuse

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/internal/api/web /src/internal/api/web

RUN go build -tags "${BUILD_TAGS}" -trimpath -ldflags="-s -w" -o /out/litepan ./cmd/litepan


FROM ${DEBIAN_IMAGE} AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata fuse3 \
    && (sed -i 's/^#user_allow_other/user_allow_other/' /etc/fuse.conf 2>/dev/null || true) \
    && (grep -q '^user_allow_other' /etc/fuse.conf || echo user_allow_other >> /etc/fuse.conf) \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN mkdir -p /app/data/log /app/strm /app/mounts

COPY --from=build /out/litepan /app/litepan
COPY --from=build /src/LICENSE /src/THIRD_PARTY_NOTICES.md /app/licenses/
COPY --from=build /src/drivers/115/client/charset/LICENSE.upstream /src/drivers/115/client/charset/LICENSE.cpython /src/drivers/115/client/charset/NOTICE /app/licenses/charset/
COPY --from=build /src/drivers/115/client/cipher/LICENSE.upstream /src/drivers/115/client/cipher/NOTICE /app/licenses/cipher/

ENV LITEPAN_DATA_DIR=/app/data \
    LITEPAN_STRM_DIR=/app/strm \
    LITEPAN_LISTEN=:5211 \
    LITEPAN_LOG_LEVEL=info \
    TZ=Asia/Shanghai

EXPOSE 5211 42069/tcp 42069/udp

VOLUME ["/app/data", "/app/strm", "/app/mounts"]

ENTRYPOINT ["/app/litepan"]
