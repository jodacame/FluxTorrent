import { ArrowDown, ArrowUp, Heart, HardDrive } from "lucide-react";
import { fmtSize, fmtSpeed } from "@/util";
import { useI18n } from "@/i18n";

export interface DiskInfo {
  available: boolean;
  totalBytes?: number;
  freeBytes?: number;
}

const REPO = "https://github.com/jodacame/fluxtorrent";
const SPONSOR = "https://github.com/sponsors/jodacame";

// Inline GitHub mark (lucide dropped brand icons).
function GitHubMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden>
      <path d="M12 .5a12 12 0 0 0-3.8 23.4c.6.1.8-.3.8-.6v-2c-3.3.7-4-1.6-4-1.6-.6-1.3-1.3-1.7-1.3-1.7-1.1-.7 0-.7 0-.7 1.2 0 1.8 1.2 1.8 1.2 1.1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.8-1.6-2.7-.3-5.5-1.3-5.5-5.9 0-1.3.5-2.4 1.2-3.2 0-.3-.5-1.5.2-3.1 0 0 1-.3 3.3 1.2a11.5 11.5 0 0 1 6 0C17.3 4.7 18.3 5 18.3 5c.7 1.6.2 2.8.1 3.1.8.8 1.2 1.9 1.2 3.2 0 4.6-2.8 5.6-5.5 5.9.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6A12 12 0 0 0 12 .5Z" />
    </svg>
  );
}

export function StatusBar({
  count,
  down,
  up,
  version,
  disk,
}: {
  count: number;
  down: number;
  up: number;
  version: string;
  disk?: DiskInfo;
}) {
  const { t } = useI18n();
  return (
    <div className="flex h-7 shrink-0 items-center gap-2 border-t bg-card px-2 text-xs text-muted-foreground sm:gap-3 sm:px-3">
      <span className="whitespace-nowrap">{t("status.torrents", { n: count })}</span>

      <div className="mx-1 hidden h-3.5 w-px bg-border sm:block" />

      <a
        href={REPO}
        target="_blank"
        rel="noreferrer"
        className="hidden items-center gap-1.5 transition-colors hover:text-foreground sm:flex"
        title="GitHub"
      >
        <GitHubMark className="size-3.5" />
        <span className="tabular">v{version}</span>
      </a>

      <a
        href={SPONSOR}
        target="_blank"
        rel="noreferrer"
        className="hidden items-center gap-1 rounded-full border border-pink-500/30 bg-pink-500/10 px-2 py-0.5 text-pink-400 transition-colors hover:bg-pink-500/20 sm:flex"
        title="GitHub Sponsors"
      >
        <Heart className="size-3 fill-current" />
        Sponsor
      </a>

      {disk?.available && (
        <span
          className="ml-auto flex items-center gap-1.5 whitespace-nowrap tabular"
          title={t("status.diskFree", {
            free: fmtSize(disk.freeBytes ?? 0),
            total: fmtSize(disk.totalBytes ?? 0),
          })}
        >
          <HardDrive className="size-3 shrink-0" />
          <span>{fmtSize(disk.freeBytes ?? 0)}</span>
          <span className="hidden text-muted-foreground/60 sm:inline">/ {fmtSize(disk.totalBytes ?? 0)}</span>
        </span>
      )}

      <span className={`${disk?.available ? "" : "ml-auto"} flex items-center gap-1 whitespace-nowrap tabular text-emerald-400`}>
        <ArrowDown className="size-3 shrink-0" /> {fmtSpeed(down)}
      </span>
      <span className="flex items-center gap-1 whitespace-nowrap tabular text-sky-400">
        <ArrowUp className="size-3 shrink-0" /> {fmtSpeed(up)}
      </span>
    </div>
  );
}
