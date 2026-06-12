# Drop-in compatibility

FluxTorrent speaks the APIs of several popular torrent-streaming servers, so an
existing client can point at FluxTorrent's host/port **unchanged**. Each layer is
a self-contained, documented module:

| Server | Module | Transparency |
|---|---|---|
| **TorrServer** (MatriX) | [`torrserver.go`](../internal/api/torrserver.go) | Full drop-in |
| **Stremio** (streaming server) | [`stremio.go`](../internal/api/stremio.go) | Full drop-in |
| **torrent2http** (Kodi/Quasar) | [`torrent2http.go`](../internal/api/torrent2http.go) | Best-effort (single-torrent API) |

---

## TorrServer (MatriX)

If a client already speaks to TorrServer, just point it at FluxTorrent's host
and port — **no client changes, no reconfiguration of stream URLs**. The same
endpoints, request bodies and response shapes are served.

## How to migrate

1. Stop TorrServer.
2. Run FluxTorrent on the same host/port the client used (default `7001`; set
   `FT_LISTEN_PORT` to match your old TorrServer port if it differs, e.g. `8090`).
3. Done. The client keeps working; saved stream URLs resolve unchanged.

```bash
# example: replace a TorrServer that ran on :8090
docker run -d -p 8090:8090 -e FT_LISTEN_PORT=8090 \
  -v ./config:/config fluxtorrent:latest
```

## Supported TorrServer endpoints

| Endpoint | Purpose | Notes |
|---|---|---|
| `GET /echo` | Server probe / version | Clients use it to detect availability |
| `POST /torrents` | Control | actions: `list`, `add`, `get`, `rem`, `drop`, `set` |
| `GET /stream/{name}?link=&index=&play` | Stream a file | `link` = infohash **or** magnet; `index` is **1-based** |
| `GET /stream?link=&index=&play` | Stream (nameless) | same semantics |
| `GET /play/{hash}/{index}` | Stream shortcut | 1-based index |
| `GET /stream/...?m3u` | M3U playlist | playable files only |
| `GET /stream/...?stat` | Torrent JSON | live stats object |
| `POST /settings` | Get/set settings | TorrServer field names (`CacheSize`, `ReaderReadAHead`) |

### Behavioural mapping

- **File indexes** — TorrServer `file_stats[].id` is 1-based; FluxTorrent's native
  API is 0-based. The compat layer converts automatically, so clients keep using
  the indexes they already store.
- **`add` by magnet or hash** — if the client streams a `link` that isn't added
  yet, FluxTorrent adds it on demand (auto-start), so the old TorrServer "add then
  stream" and "stream a magnet directly" flows both work.
- **`stat`** — engine states map to TorrServer's numeric `stat`
  (2 getting-info, 3 preload, 4 working).
- **Range / seeking** — every stream route is HTTP Range-capable (`206`), so
  seeking in mpv/VLC/Infuse/web players behaves exactly as before.

## Clients known to work unchanged

Anything that targets the TorrServer (MatriX) HTTP API, including:

- **LumoraTV** (the companion app)
- **TorrServe** / **TorrServe-ktor** (Android)
- **Lampa** and TorrServer-based add-ons
- Custom scripts using `/echo`, `/torrents`, `/stream`

## Stremio (streaming server)

Stremio's local streaming server serves torrents from **root-level paths**
(`GET /{infoHash}/{fileIdx}`, 0-based). FluxTorrent matches these inside its root
handler, so they coexist with the embedded UI without collisions.

| Endpoint | Purpose |
|---|---|
| `GET /{infoHash}/{fileIdx}` | Stream a file (0-based, Range-capable). Adds by infohash (DHT) if not present. |
| `GET\|POST /{infoHash}/create` | Ensure the torrent is added; returns its stats |
| `GET /{infoHash}/stats.json` | Torrent stats JSON |
| `GET /stats.json` | Global stats JSON |

Transcoding routes (`/hlsv2`, `/transcode`) are intentionally **not** supported —
FluxTorrent never transcodes; the player handles codecs.

## torrent2http (Kodi / Quasar)

torrent2http is a **single-torrent-per-process** bridge: its requests carry no
torrent identifier. In a persistent multi-torrent service that can't be perfectly
transparent, so this layer operates on a **selected** torrent — the `?hash=` query
if given, otherwise the most recently added active torrent.

| Endpoint | Purpose |
|---|---|
| `GET /status` | Session/torrent status JSON |
| `GET /ls` | File list JSON (each file carries a `/get/<index>` URL) |
| `GET /files/{path}` | Stream a file by its path |
| `GET /get/{index}` | Stream a file by 0-based index |

> For multiple concurrent torrents through the torrent2http layer, pass
> `?hash=<infohash>` so the right one is targeted.

## Other servers / generic clients

For clients that aren't covered above, the **generic link-based stream** is the
lowest-common-denominator contract most torrent-streaming players understand:

```
GET /stream/<anything>?link=<magnet-or-infohash>&index=<1-based>&play
```

Paste a magnet, get a seekable HTTP URL.

## Native API

New integrations should prefer the native, documented API (`/api/*`,
0-based indexes, WebSocket stats) described in the [README](../README.md#api). The
compatibility layer exists so you never *have* to migrate a client to adopt
FluxTorrent.
