// API client + types mirroring the Go backend (SPEC §7).

export interface FileInfo {
  index: number;
  path: string;
  sizeBytes: number;
  playable: boolean;
}

export interface Stats {
  peers: number;
  seeders: number;
  downKbps: number;
  upKbps: number;
  progress: number;
  cacheFillMB: number;
  state: string;
  ratio: number;
  uploadedBytes: number;
}

export interface StreamClient {
  addr: string;
  agent: string;
  fileIndex: number;
  file: string;
  since: number;
  sendKbps: number;
}

export interface PeerInfo {
  addr: string;
  client: string;
  seeder: boolean;
  downKbps: number;
}

export interface TorrentInfo {
  hash: string;
  name: string;
  sizeBytes: number;
  files: FileInfo[];
  stats: Stats;
  storageMode: string; // "ram" | "disk"
  addedAt: number;
  kind: string; // "stream" | "seeding"
  private: boolean;
  keepSeed: boolean;
  seedTargetRatio: number;
  seedTargetMinutes: number;
  seedElapsedMin: number;
  clients: StreamClient[];
  peers: PeerInfo[];
  trackers: string[];
}

export interface Settings {
  cache: { mode: string; sizeMB: number; path: string; readaheadMB: number };
  seed: {
    enabled: boolean;
    dropAfterPlayback: boolean;
    maxRatio: number;
    maxMinutes: number;
    privateAuto: boolean;
    privateMaxRatio: number;
    privateMaxMinutes: number;
  };
  net: {
    listenHost: string;
    listenPort: number;
    btPort: number;
    dht: boolean;
    maxConns: number;
    downKbps: number;
    upKbps: number;
    encryptHeaders: boolean;
    ipv6: boolean;
    utp: boolean;
    disconnectTimeoutSec: number;
  };
  limits: { maxActiveTorrents: number };
  compressed: { reject: boolean };
  disk: {
    maxGB: number;
    graceMinutes: number;
    deleteAfterSeed: boolean;
    deleteAfterPlayback: boolean;
  };
  noPeersTimeoutSec: number;
  apiToken: string;
}

export interface AddResult {
  hash: string;
  name: string;
  files: FileInfo[];
  playable: boolean;
  warnings: string[];
}

export type RuleField = "indexer" | "tracker" | "name";
export type RuleOp = "equals" | "contains" | "regex";
export type RuleAction = "reject" | "prefer" | "forceDisk" | "forceRam" | "keepSeed";

export interface Rule {
  match: { field: RuleField; op: RuleOp; value: string };
  action: RuleAction;
  seed?: { maxRatio: number; maxMinutes: number };
  maxConns?: number;
  note: string;
}

// Notified when an authenticated request comes back 401 (session expired/missing),
// so the app can drop back to the login screen.
let onUnauthorized: (() => void) | null = null;
export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn;
}

async function j<T>(res: Response): Promise<T> {
  if (!res.ok) {
    if (res.status === 401) onUnauthorized?.();
    let msg = `${res.status}`;
    try {
      const body = await res.json();
      msg = body.error || msg;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

export interface AuthStatus {
  required: boolean; // a UI password is configured
  authenticated: boolean; // this session is logged in (or auth is off)
}

export const api = {
  list: () => fetch("/api/torrents").then((r) => j<TorrentInfo[]>(r)),
  add: (link: string) =>
    fetch("/api/torrents", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ link }),
    }).then((r) => j<AddResult>(r)),
  drop: (hash: string) => fetch(`/api/torrents/${hash}/drop`, { method: "POST" }),
  remove: (hash: string, withFiles: boolean) =>
    fetch(`/api/torrents/${hash}${withFiles ? "?withFiles=true" : ""}`, { method: "DELETE" }),
  getSettings: () => fetch("/api/settings").then((r) => j<Settings>(r)),
  putSettings: (s: Settings) =>
    fetch("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(s),
    }).then((r) => j<Settings>(r)),
  getRules: () => fetch("/api/rules").then((r) => j<Rule[]>(r)),
  putRules: (rules: Rule[]) =>
    fetch("/api/rules", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(rules),
    }).then((r) => j<Rule[]>(r)),
  health: () => fetch("/api/health").then((r) => j<{ version: string; uptime: number; activeTorrents: number; cacheMode: string }>(r)),
  disk: () =>
    fetch("/api/disk").then((r) =>
      j<{ path: string; available: boolean; totalBytes?: number; freeBytes?: number; usedBytes?: number }>(r)
    ),
  streamUrl: (hash: string, index: number) =>
    `${location.origin}/stream/${hash}/${index}`,

  authStatus: () => fetch("/api/auth").then((r) => j<AuthStatus>(r)),
  login: async (password: string) => {
    const res = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    });
    if (!res.ok) {
      let msg = `${res.status}`;
      try {
        msg = (await res.json()).error || msg;
      } catch {
        /* ignore */
      }
      throw new Error(msg);
    }
  },
  logout: () => fetch("/api/logout", { method: "POST" }),
};

export type WsEvent =
  | { type: "stats"; torrents: TorrentInfo[] }
  | { type: "added"; hash: string; name: string }
  | { type: "dropped"; hash: string }
  | { type: "warning"; hash: string; name: string };

// connectEvents opens the live WebSocket and auto-reconnects.
export function connectEvents(onEvent: (e: WsEvent) => void): () => void {
  let ws: WebSocket | null = null;
  let closed = false;
  let retry: ReturnType<typeof setTimeout>;

  const open = () => {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    ws = new WebSocket(`${proto}://${location.host}/api/events`);
    ws.onmessage = (m) => {
      try {
        onEvent(JSON.parse(m.data));
      } catch {
        /* ignore */
      }
    };
    ws.onclose = () => {
      if (!closed) retry = setTimeout(open, 2000);
    };
    ws.onerror = () => ws?.close();
  };
  open();

  return () => {
    closed = true;
    clearTimeout(retry);
    ws?.close();
  };
}
