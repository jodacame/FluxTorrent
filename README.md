<div align="center">

# ◖ FluxTorrent

**A simple, efficient, self-hosted torrent → HTTP streaming bridge with a web UI.**

Turn a magnet/infohash into a seekable HTTP stream on demand for any media player
(mpv, VLC, Infuse…) — focused, modern, **private-tracker friendly**, and **compatible with the
APIs your clients already speak** (TorrServer, Stremio, torrent2http).

[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](#-quick-start-docker-recommended)
[![Sponsor](https://img.shields.io/badge/Sponsor-❤-ec4899)](https://github.com/sponsors/jodacame)

![FluxTorrent UI](docs/screenshots/library.png)

</div>

---

## What is it?

FluxTorrent does exactly one job and does it well: it turns a **torrent into a seekable
HTTP URL on demand**, with smart piece prioritization so playback starts fast and seeking
doesn't stall. It's a *thin streaming engine*, not a download manager.

You paste a magnet link, FluxTorrent fetches just the pieces your player needs (around the
current playback position), and exposes a standard **HTTP Range** endpoint any video player
understands. Bounded memory, smart seeding, per-source rules, and a clean web UI round it out.

**Design goals:** robust, efficient, single-purpose, simple. Every feature serves streaming
playback, on principles that keep a small always-on box happy: idempotent start (a torrent is
either active or fully gone — never a stuck *"finding peers… 0/0"* zombie), a **bounded RAM
ring buffer** so memory stays predictable, drop-after-playback so torrents don't pile up, and a
no-peers timeout that fails fast instead of hanging.

It stands on the shoulders of the projects that defined this space —
**[TorrServer](https://github.com/YouROK/TorrServer)**, **[Stremio](https://www.stremio.com/)**
and **torrent2http** — and stays **API-compatible** with them so it fits the clients people
already use.

## 🔒 Private-first

Most torrent streamers treat private trackers as an afterthought. FluxTorrent is built to be a
**good citizen on private trackers** out of the box:

- **Auto-detects private torrents** (the BEP27 `private` flag) — no per-tracker setup needed.
- **Smart seeding to satisfy ratio/time** — automatically seeds a private torrent until it
  reaches your target **ratio _or_ seed time** (whichever first), then releases it. This is the
  classic hit-and-run rule, handled for you. Progress is persisted, so seeding survives restarts.
- **Forces disk storage** for private torrents (you can't seed what RAM discards) and downloads
  the full content so there's something to share.
- **No leaks** — private torrents never use DHT/PEX/LSD (enforced by the protocol), and you can
  **force header encryption** to get past torrent-blocking ISPs.
- **Per-source rules** can fine-tune any source further (seed targets, storage mode, connection
  caps), useful when a private tracker has specific requirements.

It's all configurable (or off) in **Settings → Private trackers**.

## Features

- 🎬 **Instant streaming** — magnet/infohash → seekable HTTP (`206 Partial Content`), fast start, smooth seeks.
- 🧠 **Bounded cache** — pure **RAM** (memory-capped ring buffer) or **disk** (configurable path).
- 🔒 **Private-tracker smart seeding** — auto-detect + seed to ratio/time, then drop. Survives restarts.
- 🧭 **Per-source rules** — `reject` / `prefer` / `forceDisk` / `forceRam` / `keepSeed` (+ per-torrent connection cap), first match wins, editable in the UI.
- 📦 **Compressed-release detection** — flags/rejects RAR/ZIP/split archives a player can't use.
- 🔌 **Drop-in compatible** — speaks **TorrServer**, **Stremio** and **torrent2http** APIs, so existing clients just re-point. ([details](docs/COMPATIBILITY.md))
- 🖥️ **Web UI** — sortable torrent table, resizable panels, detail tabs, live WebSocket stats, **dark theme**, **multi-language (es/en, extensible)**.
- 👁️ **Full visibility** — mode (streaming vs sharing), rule target, source (seeders) and **which client is playing** (player + IP), at a glance.
- 🚦 **Speed & network control** — global download/upload caps, connection limit, DHT/IPv6/µTP, header encryption, idle disconnect timeout.
- 🪶 **Single binary, single container** — UI embedded via `go:embed`, ~25 MB image, ultra-light at idle, graceful shutdown.

|  |  |
|---|---|
| ![Rules](docs/screenshots/rules.png) | ![Settings](docs/screenshots/settings.png) |
| **Per-source rules editor** | **Approachable settings** |

## How it compares

An honest look at where FluxTorrent fits. These are all good tools with different goals —
FluxTorrent is a **focused single-purpose engine**, not a media center.

| | **FluxTorrent** | TorrServer | Stremio (server) | torrent2http / Peerflix |
|---|:---:|:---:|:---:|:---:|
| Primary focus | Torrent→HTTP **streaming engine** | Torrent→HTTP streaming server | Media center + streaming server | Minimal torrent→HTTP CLI |
| Bounded RAM cache | ✅ ring buffer | ✅ | ⚠️ disk/temp | ❌ |
| Disk mode + seeding | ✅ | ✅ | ✅ | partial |
| **Private-tracker smart seeding** (ratio/time, auto) | ✅ | ❌ | ❌ | ❌ |
| **Per-source rules** | ✅ | ❌ | ❌ | ❌ |
| Modern web UI | ✅ | ⚠️ basic | ✅ (full app) | ❌ |
| Speaks **other servers' APIs** | ✅ TorrServer · Stremio · torrent2http | — | — | — |
| See the playing client (player + IP) | ✅ | ❌ | ❌ | ❌ |
| Transcoding | ❌ *(by design)* | ❌ | ✅ | ❌ |
| Catalogs / indexers / addons | ❌ *(by design)* | ❌ | ✅ | ❌ |
| Single binary / small container | ✅ ~25 MB | ✅ | ⚠️ bundled with app | ✅ |
| Ecosystem & maturity | 🆕 new, small | 🏆 large, battle-tested | 🏆 huge | mature but niche |

**Be honest with yourself about what you need:**

- Want a **full media center** with catalogs, addons and transcoding? → **Stremio**.
- Want the **most battle-tested** option with the widest Android/Kodi client support and a big
  community? → **TorrServer**.
- Want a **focused, memory-bounded engine** that's a good citizen on **private trackers**, has
  **per-source rules** and a clean UI, and **drops into clients built for the tools above**?
  → that's where **FluxTorrent** aims to shine. It's newer and its ecosystem is smaller — that's
  the honest trade-off.

## 🚀 Quick start (Docker, recommended)

```bash
docker run -d --name fluxtorrent \
  -p 7001:7001 \
  -p 42069:42069 \
  -v "$PWD/config:/config" \
  -v "$PWD/downloads:/downloads" \
  -e FT_CACHE_MODE=ram \
  --restart unless-stopped \
  ghcr.io/jodacame/fluxtorrent:latest
```

Open **http://localhost:7001**, paste a magnet link, press play. Point any player at
`http://<host>:7001/stream/<hash>/<index>`.

### docker-compose (with persistence)

```yaml
services:
  fluxtorrent:
    image: ghcr.io/jodacame/fluxtorrent:latest   # or build: .
    container_name: fluxtorrent
    ports:
      - "7001:7001"        # API + UI + stream (bound 0.0.0.0)
      - "42069:42069"      # BitTorrent (optional, for incoming peers)
    volumes:
      - ./config:/config         # ← settings, rules, seed/ratio state (bbolt db)
      - ./downloads:/downloads   # ← downloaded files (disk-cache mode only)
    environment:
      - FT_CACHE_MODE=ram        # ram = stream only · disk = save to disk
      - FT_CACHE_SIZE_MB=1024
      - FT_API_TOKEN=            # optional bearer token
    restart: unless-stopped
```

```bash
docker compose up -d
```

### Making your data persistent

FluxTorrent keeps **all state in two folders** — mount them as volumes and your setup
survives restarts, upgrades and re-creates:

| Volume | Holds | Needed when |
|---|---|---|
| `/config` | bbolt DB: **settings, rules, seed/ratio state, torrent list** | always (recommended) |
| `/downloads` | the actual media files | only in **disk** cache mode |

In **RAM mode** nothing is written to `/downloads` (pure streaming). In **disk mode** files
persist there, which is what enables seeding and resume. Your `keepSeed` ratio/time progress
lives in `/config`, so private-tracker seeding continues across restarts.

> **Migrating from another server?** Run FluxTorrent on the same port your client used
> (`-e FT_LISTEN_PORT=8090 -p 8090:8090`) and existing TorrServer/Stremio clients keep working
> unchanged — saved stream URLs resolve as-is. See [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

## Configuration

Everything is editable in the **Settings** screen (with a built-in help guide) and persisted to
`/config`. It can also be booted from environment variables:

| Env var | Default | Description |
|---|---|---|
| `FT_LISTEN_HOST` | `0.0.0.0` | Bind address |
| `FT_LISTEN_PORT` | `7001` | API + UI + stream port |
| `FT_BT_PORT` | `42069` | Incoming BitTorrent port |
| `FT_CACHE_MODE` | `ram` | `ram` (stream only) or `disk` (save + seed) |
| `FT_CACHE_SIZE_MB` | `1024` | RAM ring-buffer cap |
| `FT_CACHE_PATH` | `/downloads` | Disk-mode storage root |
| `FT_READAHEAD_MB` | `64` | Bytes prioritized ahead of playback |
| `FT_API_TOKEN` | _(empty)_ | Optional `Authorization: Bearer <token>` for `/api/*` |
| `FT_CONFIG_DIR` | `/config` | Where the bbolt DB lives |

From the UI you can also tune: download/upload speed limits, max active torrents, no-peers
timeout, seeding defaults, the disk folder, and an **Advanced** section — DHT, connection
limit, **force header encryption**, IPv6, µTP, and an idle **disconnect timeout**.

### Per-source rules

Rules are evaluated **in order, first match wins** (edit them in the **Rules** screen):

- **`keepSeed`** — download fully, save to disk, and seed until `maxRatio` **or** `maxMinutes`
  is reached (whichever first; `0` = ∞), then drop. Ideal for private trackers.
- **`forceDisk` / `forceRam`** — override the storage mode for matching torrents.
- **`reject`** — refuse a flaky source (with a reason returned to the client).
- **`prefer`** — bias peer selection / keep-alive priority.
- Any rule can also set a **per-torrent connection cap** (override), handy for private trackers.

Anything that matches no rule simply **streams in RAM** and is dropped after playback.

## Compatibility

The torrent-streaming space has excellent prior art, and rather than reinvent client protocols
FluxTorrent **implements the APIs those projects established** — so the players and front-ends
already built for them work here unchanged:

| Server | Transparency | Notes |
|---|---|---|
| **TorrServer** (MatriX) | Full | `/echo`, `/torrents`, `/stream`, `/play`, `/settings` |
| **Stremio** (streaming server) | Full | root-level `/{infoHash}/{fileIdx}`, `/create`, `/stats.json` |
| **torrent2http** (Kodi/Quasar) | Best-effort | single-torrent API; targets `?hash=` or the latest torrent |

Full endpoint mapping and the list of clients that work unchanged:
**[docs/COMPATIBILITY.md](docs/COMPATIBILITY.md)**.

## API

Base: `http://<host>:7001`. Native, documented surface (prefer this for new integrations):

```
POST   /api/torrents        { "link": "magnet:…" | "infohash" }
GET    /api/torrents        → list with live stats
GET    /api/torrents/:hash  → one torrent (stats, files, clients)
POST   /api/torrents/:hash/drop
DELETE /api/torrents/:hash  (?withFiles=true)
GET    /stream/:hash/:index → Range-capable video bytes  ← players
GET/PUT /api/settings
GET/PUT /api/rules
WS     /api/events          → live { type:"stats", torrents:[…] }
GET    /api/health
```

The stream URL is **stable and token-free** so saved player URLs survive restarts.

## Building from source

Requires Go 1.23+ and Node 18+.

```bash
# 1. build the embedded UI
cd web && npm install && npm run build && cd ..

# 2. build the single binary (UI baked in)
go build -o fluxtorrent ./cmd/fluxtorrent

# 3. run
FT_CONFIG_DIR=./config ./fluxtorrent      # → http://localhost:7001
```

Or just `docker build -t fluxtorrent .` (multi-stage: Node builds the UI → Go embeds it →
tiny Alpine runtime).

## Tech stack

Go + [anacrolix/torrent](https://github.com/anacrolix/torrent) engine · React + Vite +
TypeScript + [shadcn/ui](https://ui.shadcn.com) UI embedded via `go:embed` · bbolt
persistence · Docker.

## Roadmap

- [x] Core engine: add / stream / drop / delete / stats, idempotent start, no zombie state
- [x] RAM ring buffer + disk mode, compressed detection, no-peers timeout
- [x] Per-source rules + `keepSeed` ratio/time enforcement
- [x] Private-tracker auto-detection & smart seeding
- [x] Web UI (sortable table, resizable panels, rules editor, settings, i18n es/en)
- [x] TorrServer / Stremio / torrent2http compatibility · speed/network controls
- [ ] More languages · per-torrent storage override in the UI · metrics endpoint

## Contributing

Contributions, issues and translations are welcome! New languages are a single file under
`web/src/i18n/locales/`. Open an issue to discuss larger changes first.

If FluxTorrent is useful to you, consider **[sponsoring](https://github.com/sponsors/jodacame)** ❤ — it helps keep the project maintained.

## Security

FluxTorrent is a **self-hosted service meant for a trusted network**. By default it
has **no authentication**, permissive CORS, and binds `0.0.0.0` — anyone who can
reach the port can control it.

- Set **`FT_API_TOKEN`** to require a bearer token on `/api/*`.
- The token only guards `/api/*`; **`/stream` and the compatibility endpoints stay
  open by design** so players work without credentials.
- To expose it publicly, keep it behind a **VPN or a reverse proxy** (TLS + auth).

Full details and how to report vulnerabilities: **[SECURITY.md](SECURITY.md)**.

## License

[Apache-2.0](LICENSE). You're free to use, modify and redistribute it under those
terms; it comes with no warranty (see [Disclaimer](#disclaimer)).

## Disclaimer

FluxTorrent is a **general-purpose streaming engine** — software that turns BitTorrent data
into an HTTP stream. It ships with **no content, no trackers, no indexers and no preconfigured
sources**, and it does not endorse or facilitate any particular use.

The software is provided **"as is", without warranty of any kind**, express or implied (see the
[LICENSE](LICENSE) for the full terms). To the maximum extent permitted by law, the authors and
contributors **accept no liability** for any claim, damages or other liability arising from the
use of this software.

**You are solely responsible** for what you add, stream, download and share with it, and for
ensuring your use complies with all applicable laws, regulations, and the rules of any tracker
or network you connect to. Downloading or sharing copyrighted material without authorization may
be illegal in your jurisdiction. Use FluxTorrent only with content you have the right to access
and distribute. By using the software you accept full responsibility for your usage.
