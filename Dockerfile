# syntax=docker/dockerfile:1

# ---- Build stage ----
# Uses Go 1.26 (pinned, no "latest") to compile a static binary.
# --platform=$BUILDPLATFORM together with TARGETOS/TARGETARCH enables
# cross-compilation for amd64 and arm64 via docker buildx.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -trimpath -o /out/gridserver ./cmd/server

# ---- Runtime stage ----
# Minimal Alpine image containing only the compiled binary and CA certs.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/gridserver /usr/local/bin/gridserver
COPY config.json /etc/gridserver/config.json
ENV GRID_CONFIG_PATH=/etc/gridserver/config.json
EXPOSE 57579
ENTRYPOINT ["gridserver"]
