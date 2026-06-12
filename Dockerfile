# FluxTorrent — multi-stage build (SPEC §11):
#   node builds the UI → go embeds it → tiny alpine runtime image.

# ---------- stage 1: build the React UI ----------
FROM node:22-alpine AS ui
WORKDIR /ui
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# ---------- stage 2: build the Go binary (UI embedded) ----------
FROM golang:1.23-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# overlay the freshly built UI so go:embed picks it up
COPY --from=ui /ui/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fluxtorrent ./cmd/fluxtorrent

# ---------- stage 3: runtime ----------
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
