# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Instead, use GitHub's private vulnerability reporting:
**Security → Report a vulnerability** on the repository
([github.com/jodacame/fluxtorrent/security/advisories/new](https://github.com/jodacame/fluxtorrent/security/advisories/new)).

We'll acknowledge your report as soon as possible and work with you on a fix and
a coordinated disclosure. Thank you for helping keep users safe.

## Supported versions

FluxTorrent is pre-1.0 — security fixes land on `main` and the latest released
image (`ghcr.io/jodacame/fluxtorrent:latest`). Please test against the latest
version before reporting.

## Security model — read this before exposing it

FluxTorrent is a **self-hosted service designed for a trusted network** (your LAN,
a VPN, or behind a reverse proxy). Understand its defaults before putting it on a
public address:

- **No authentication by default.** Anyone who can reach the port can add, stream
  and delete torrents. Set `FT_API_TOKEN` to require `Authorization: Bearer <token>`
  on the `/api/*` endpoints.
- **The token only protects `/api/*`.** The streaming endpoint (`/stream/...`) and
  the compatibility endpoints (TorrServer `/echo`,`/torrents`,`/stream`,`/play`,`/settings`;
  Stremio `/{infoHash}/...`; torrent2http `/status`,`/ls`,`/get`,`/files`) stay
  **open by design** so media players can reach them without embedding credentials.
  If you need those locked down, put FluxTorrent behind a reverse proxy that
  enforces auth/allowlists.
- **CORS is permissive (`*`)** and the server **binds `0.0.0.0`** by default so it's
  reachable on your network. Restrict exposure at the network layer.
- **The container runs as root** for bind-mount volume compatibility. You can drop
  privileges with `--user`/compose `user:` after `chown`-ing the `config`/`downloads`
  volumes.
- **The optional API token is stored in the bbolt DB** under `/config` in plaintext.
  Keep that volume private.

### Exposing it safely (recommended)

1. Keep it on your **LAN or a VPN** (WireGuard/Tailscale) whenever possible.
2. If it must be public, put it **behind a reverse proxy** (Caddy/Traefik/nginx)
   that terminates TLS and enforces authentication and IP allowlists.
3. Set **`FT_API_TOKEN`** so the control API isn't open.
4. Only stream content you have the right to access (see the project disclaimer).

## Scope

FluxTorrent connects to the public BitTorrent network: it talks to untrusted peers
and trackers. The engine ([anacrolix/torrent](https://github.com/anacrolix/torrent))
handles that surface; report engine-level issues upstream where appropriate. Issues
in FluxTorrent's own API, UI, compatibility layers or configuration handling are in
scope here.
