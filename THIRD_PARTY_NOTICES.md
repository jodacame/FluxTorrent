# Third-party notices

FluxTorrent (Apache-2.0) is built on open-source components. This file provides
attribution and license information for the main dependencies. Full license texts
ship with each dependency in its source repository / module.

No dependency is licensed under GPL, LGPL or AGPL.

## Backend (Go)

| Component | License | Source |
|---|---|---|
| github.com/anacrolix/torrent (and anacrolix/* dht, log, generics, mmsg, utp, …) | **MPL-2.0** | https://github.com/anacrolix/torrent |
| github.com/gorilla/websocket | BSD-2-Clause | https://github.com/gorilla/websocket |
| go.etcd.io/bbolt | MIT | https://github.com/etcd-io/bbolt |
| github.com/pion/* (webrtc stack) | MIT | https://github.com/pion |
| modernc.org/sqlite | BSD-3-Clause | https://gitlab.com/cznic/sqlite |
| lukechampine.com/blake3 | MIT | https://github.com/lukechampine/blake3 |
| golang.org/x/* | BSD-3-Clause | https://cs.opensource.google/go/x |

### MPL-2.0 notice

Some components (the anacrolix BitTorrent engine and related modules) are licensed
under the **Mozilla Public License 2.0**. Their source is available at the URLs
above. FluxTorrent uses these components **unmodified**; per MPL-2.0, no source
disclosure of FluxTorrent's own (Apache-2.0) code is required. If you modify those
MPL-covered files, you must make the modified files' source available under MPL-2.0.

## Frontend (npm)

The UI uses React, Radix UI / shadcn/ui, Tailwind CSS, lucide icons and supporting
libraries. Their licenses across the dependency tree are: **MIT** (majority),
Apache-2.0, ISC, MPL-2.0, BSD-3-Clause, 0BSD and CC-BY-4.0. None are GPL/AGPL/LGPL.

| Component | License |
|---|---|
| react, react-dom | MIT |
| @radix-ui/* | MIT |
| tailwindcss, tw-animate-css | MIT |
| lucide-react | ISC |
| class-variance-authority, clsx, tailwind-merge | MIT / Apache-2.0 |
| sonner, react-resizable-panels | MIT |

---

This list covers the principal dependencies. A complete, machine-generated
inventory can be produced with `go-licenses` (Go) and `license-checker` (npm).
