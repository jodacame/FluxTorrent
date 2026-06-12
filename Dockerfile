# FluxTorrent — multi-stage build:
#   node builds the UI → go cross-compiles & embeds it → tiny alpine runtime.
#
# The UI and Go stages run on the native BUILDPLATFORM and Go cross-compiles to
# the TARGET arch (CGO disabled, pure Go), so multi-arch images build fast
# without QEMU emulation.

# ---------- stage 1: build the React UI (native) ----------
FROM --platform=$BUILDPLATFORM node:22-alpine AS ui
WORKDIR /ui
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# ---------- stage 2: cross-compile the Go binary (UI embedded) ----------
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# overlay the freshly built UI so go:embed picks it up
COPY --from=ui /ui/dist ./web/dist
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/fluxtorrent ./cmd/fluxtorrent

# ---------- stage 3: runtime (target arch) ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/fluxtorrent /usr/local/bin/fluxtorrent
# Runs as root so bind-mounted /config and /downloads "just work" regardless of
# host ownership (standard for self-hosted media containers). Drop privileges
# with `--user` / compose `user:` after chowning the volumes if you prefer.
EXPOSE 7001 42069
ENV FT_CONFIG_DIR=/config FT_LISTEN_HOST=0.0.0.0 FT_LISTEN_PORT=7001
VOLUME ["/config", "/downloads"]
ENTRYPOINT ["/usr/local/bin/fluxtorrent"]
