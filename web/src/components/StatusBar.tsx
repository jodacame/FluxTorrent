import { ArrowDown, ArrowUp, Heart } from "lucide-react";
import { fmtSpeed } from "@/util";
import { useI18n } from "@/i18n";

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
}: {
  count: number;
  down: number;
  up: number;
  version: string;
}) {
  const { t } = useI18n();
  return (
    <div className="flex h-7 shrink-0 items-center gap-3 border-t bg-card px-3 text-xs text-muted-foreground">
      <span>{t("status.torrents", { n: count })}</span>

      <div className="mx-1 h-3.5 w-px bg-border" />

      <a
        href={REPO}
        target="_blank"
        rel="noreferrer"
        className="flex items-center gap-1.5 transition-colors hover:text-foreground"
        title="GitHub"
      >
        <GitHubMark className="size-3.5" />
        <span className="tabular">v{version}</span>
      </a>

      <a
        href={SPONSOR}
        target="_blank"
        rel="noreferrer"
        className="flex items-center gap-1 rounded-full border border-pink-500/30 bg-pink-500/10 px-2 py-0.5 text-pink-400 transition-colors hover:bg-pink-500/20"
        title="GitHub Sponsors"
      >
        <Heart className="size-3 fill-current" />
        Sponsor
      </a>

      <span className="ml-auto flex items-center gap-1 tabular text-emerald-400">
        <ArrowDown className="size-3" /> {fmtSpeed(down)}
      </span>
      <span className="flex items-center gap-1 tabular text-sky-400">
        <ArrowUp className="size-3" /> {fmtSpeed(up)}
      </span>
    </div>
  );
}
