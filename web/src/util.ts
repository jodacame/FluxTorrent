// Human-friendly mappers (SPEC §11b: states as human language, not raw numbers).

export interface StateView {
  key: string; // i18n key, resolved by the caller via t()
  tone: "good" | "busy" | "warn" | "idle";
  pulse: boolean;
}

// stateView maps an engine state to an i18n key + visual tone (SPEC §11b).
export function stateView(state: string): StateView {
  switch (state) {
    case "playing":
      return { key: "state.playing", tone: "good", pulse: true };
    case "ready":
      return { key: "state.ready", tone: "good", pulse: false };
    case "seeding":
      return { key: "state.seeding", tone: "good", pulse: true };
    case "downloading":
      return { key: "state.downloading", tone: "busy", pulse: true };
    case "fetching":
      return { key: "state.fetching", tone: "busy", pulse: true };
    case "searching":
      return { key: "state.searching", tone: "warn", pulse: true };
    default:
      return { key: state, tone: "idle", pulse: false };
  }
}

export function fmtSize(bytes: number): string {
  if (bytes <= 0) return "—";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let n = bytes;
  while (n >= 1024 && i < u.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
}

export function fmtSpeed(kbps: number): string {
  if (kbps <= 0) return "0 KB/s";
  const mbps = kbps / 8 / 1000; // kbps (bits) → MB/s
  if (mbps >= 1) return `${mbps.toFixed(1)} MB/s`;
  return `${Math.round((kbps / 8))} KB/s`;
}

// fmtMins turns a minute count into a compact "Xh Ym" / "Ym" / "<1m" label.
export function fmtMins(mins: number): string {
  if (mins <= 0) return "<1m";
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`;
  return `${m}m`;
}

export function shortName(name: string): string {
  return name.replace(/\.(mkv|mp4|avi|m4v|ts|webm|mov)$/i, "");
}

// friendlyAgent turns a User-Agent string into a recognizable player name.
export function friendlyAgent(ua: string): string {
  if (!ua) return "Reproductor";
  const map: [RegExp, string][] = [
    [/VLC/i, "VLC"],
    [/mpv/i, "mpv"],
    [/Infuse/i, "Infuse"],
    [/Stremio/i, "Stremio"],
    [/Kodi|XBMC/i, "Kodi"],
    [/ExoPlayer|TorrServe/i, "TorrServe"],
    [/Lavf|libav|ffmpeg/i, "FFmpeg"],
    [/AppleCoreMedia/i, "Apple"],
    [/Chrome/i, "Chrome"],
    [/Firefox/i, "Firefox"],
    [/Safari/i, "Safari"],
    [/curl|wget/i, "CLI"],
  ];
  for (const [re, name] of map) if (re.test(ua)) return name;
  return ua.split("/")[0].slice(0, 18) || "Reproductor";
}

// hostOf strips the port from an addr (IP:port → IP).
export function hostOf(addr: string): string {
  const i = addr.lastIndexOf(":");
  return i > 0 ? addr.slice(0, i) : addr;
}
