import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  Plus,
  Play,
  Link2,
  Pause,
  Trash2,
  Loader2,
  Menu,
  X,
  LogOut,
} from "lucide-react";
import { api, connectEvents, type TorrentInfo } from "./api";
import { copyText, shortName } from "./util";
import { useI18n } from "./i18n";
import { useIsDesktop } from "@/hooks/use-media-query";
import { useDefaultLayout } from "react-resizable-panels";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Sheet, SheetClose, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { Sidebar, type Filter } from "@/components/Sidebar";
import { TorrentTable } from "@/components/TorrentTable";
import { DetailPanel } from "@/components/DetailPanel";
import { RulesEditor } from "@/components/RulesEditor";
import { SettingsView } from "@/components/SettingsView";
import { StatusBar, type DiskInfo } from "@/components/StatusBar";

export type View = "library" | "rules" | "settings";

export default function App({
  authEnabled = false,
  onLogout,
}: {
  authEnabled?: boolean;
  onLogout?: () => void;
} = {}) {
  const { t } = useI18n();
  const [torrents, setTorrents] = useState<TorrentInfo[]>([]);
  const [view, setView] = useState<View>("library");
  const [filter, setFilter] = useState<Filter>("all");
  const [selected, setSelected] = useState<string | null>(null);
  const [link, setLink] = useState("");
  const [adding, setAdding] = useState(false);
  const [version, setVersion] = useState("");
  const [disk, setDisk] = useState<DiskInfo | undefined>();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const isDesktop = useIsDesktop();

  // persist panel layouts across reloads (localStorage)
  const hLayout = useDefaultLayout({ id: "ft-layout-h", storage: localStorage });
  const vLayout = useDefaultLayout({ id: "ft-layout-v", storage: localStorage });

  const ingest = useCallback((list: TorrentInfo[]) => {
    const rank = (x: TorrentInfo) =>
      x.stats.state === "searching" ? 0 : x.stats.state === "fetching" ? 1 : 2;
    setTorrents([...list].sort((a, b) => rank(a) - rank(b) || b.addedAt - a.addedAt));
  }, []);

  useEffect(() => {
    api.health().then((h) => setVersion(h.version)).catch(() => {});
    api.list().then(ingest).catch(() => {});
    const off = connectEvents((e) => {
      if (e.type === "stats") ingest(e.torrents);
      else if (e.type === "dropped") api.list().then(ingest).catch(() => {});
    });
    return off;
  }, [ingest]);

  // Disk usage refreshes on a slow cadence — free space changes gradually.
  useEffect(() => {
    const pull = () => api.disk().then(setDisk).catch(() => {});
    pull();
    const id = setInterval(pull, 15000);
    return () => clearInterval(id);
  }, []);

  const add = async () => {
    const v = link.trim();
    if (!v || adding) return;
    setAdding(true);
    try {
      const res = await api.add(v);
      setLink("");
      if (res.warnings?.length) toast.warning(res.warnings[0]);
      else toast.success(t("toast.added", { name: shortName(res.name) }));
      api.list().then(ingest).catch(() => {});
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setAdding(false);
    }
  };

  const sel = useMemo(() => torrents.find((x) => x.hash === selected) ?? null, [torrents, selected]);

  const play = (x: TorrentInfo) => {
    const f = x.files.find((y) => y.playable) ?? x.files[0];
    if (f) window.open(api.streamUrl(x.hash, f.index), "_blank");
  };
  const copy = async (x: TorrentInfo) => {
    const f = x.files.find((y) => y.playable) ?? x.files[0];
    if (!f) return;
    const ok = await copyText(api.streamUrl(x.hash, f.index));
    if (ok) toast.success(t("toast.copied"));
    else toast.error(t("toast.copyFailed"));
  };
  const remove = (x: TorrentInfo, withFiles = false) => {
    const msg = t(withFiles ? "confirm.deleteFiles" : "confirm.delete", { name: shortName(x.name) });
    if (confirm(msg)) api.remove(x.hash, withFiles);
  };

  const filtered = useMemo(() => {
    switch (filter) {
      case "streaming":
        return torrents.filter((x) => x.kind === "stream");
      case "sharing":
        return torrents.filter((x) => x.kind === "seeding");
      case "downloading":
        return torrents.filter((x) => x.stats.progress < 0.999);
      case "completed":
        return torrents.filter((x) => x.stats.progress >= 0.999);
      default:
        return torrents;
    }
  }, [torrents, filter]);

  const totals = useMemo(() => {
    const down = torrents.reduce((a, x) => a + x.stats.downKbps, 0);
    const up = torrents.reduce((a, x) => a + x.stats.upKbps, 0);
    return { count: torrents.length, down, up };
  }, [torrents]);

  const counts = {
    all: torrents.length,
    streaming: torrents.filter((x) => x.kind === "stream").length,
    sharing: torrents.filter((x) => x.kind === "seeding").length,
    downloading: torrents.filter((x) => x.stats.progress < 0.999).length,
    completed: torrents.filter((x) => x.stats.progress >= 0.999).length,
  };

  const sidebar = (onNavigate?: () => void) => (
    <Sidebar
      view={view}
      onView={(v) => {
        setView(v);
        onNavigate?.();
      }}
      filter={filter}
      onFilter={(f) => {
        setFilter(f);
        onNavigate?.();
      }}
      counts={counts}
    />
  );

  const mainView =
    view === "rules" ? (
      <RulesEditor />
    ) : view === "settings" ? (
      <SettingsView />
    ) : isDesktop ? (
      <ResizablePanelGroup
        orientation="vertical"
        defaultLayout={vLayout.defaultLayout}
        onLayoutChanged={vLayout.onLayoutChanged}
      >
        <ResizablePanel id="table" defaultSize="66%" minSize="25%">
          <TorrentTable
            torrents={filtered}
            selected={selected}
            onSelect={setSelected}
            onPlay={play}
            onCopy={copy}
            onDrop={(x) => api.drop(x.hash)}
            onDelete={remove}
          />
        </ResizablePanel>
        <ResizableHandle withHandle />
        <ResizablePanel id="detail" defaultSize="34%" minSize="0%" collapsible>
          <DetailPanel torrent={sel} />
        </ResizablePanel>
      </ResizablePanelGroup>
    ) : (
      <TorrentTable
        compact
        torrents={filtered}
        selected={selected}
        onSelect={setSelected}
        onPlay={play}
        onCopy={copy}
        onDrop={(x) => api.drop(x.hash)}
        onDelete={remove}
      />
    );

  return (
    <div className="flex h-screen flex-col bg-background text-foreground">
      {/* toolbar */}
      <div className="flex shrink-0 items-center gap-2 border-b bg-card/60 px-2 py-2 sm:px-3">
        <Button
          variant="ghost"
          size="icon"
          className="size-8 shrink-0 lg:hidden"
          onClick={() => setSidebarOpen(true)}
          aria-label={t("nav.menu")}
        >
          <Menu className="size-5" />
        </Button>

        <div className="flex shrink-0 select-none items-center gap-1.5 pr-1 text-[15px]">
          <BrandMark className="size-4 text-primary" />
          <span className="text-xs font-bold sm:hidden">F<b className="text-primary">T</b></span>
          <span className="hidden font-medium sm:inline">Flux<b className="text-primary">Torrent</b></span>
        </div>
        <Separator orientation="vertical" className="mx-1 hidden h-6 sm:block" />

        <div className="flex min-w-0 flex-1 items-center gap-2 sm:max-w-md">
          <div className="relative flex-1">
            <Link2 className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={link}
              onChange={(e) => setLink(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && add()}
              placeholder={t("add.placeholder")}
              className="h-8 pl-8"
            />
          </div>
          <Button size="sm" onClick={add} disabled={adding || !link.trim()} className="h-8 shrink-0">
            {adding ? <Loader2 className="animate-spin" data-icon="inline-start" /> : <Plus data-icon="inline-start" />}
            <span className="hidden sm:inline">{t("add.button")}</span>
          </Button>
        </div>

        <Separator orientation="vertical" className="mx-1 hidden h-6 lg:block" />

        <div className="hidden items-center gap-2 lg:flex">
          <ToolbarAction label={t("menu.play")} disabled={!sel} onClick={() => sel && play(sel)} icon={Play} />
          <ToolbarAction label={t("menu.copy")} disabled={!sel} onClick={() => sel && copy(sel)} icon={Link2} />
          <ToolbarAction label={t("menu.drop")} disabled={!sel} onClick={() => sel && api.drop(sel.hash)} icon={Pause} />
          <ToolbarAction label={t("menu.delete")} disabled={!sel} onClick={() => sel && remove(sel)} icon={Trash2} danger />
        </div>

        <div className="ml-auto hidden items-center gap-1.5 pr-1 text-xs text-muted-foreground md:flex">
          <span className="size-1.5 rounded-full bg-emerald-400 shadow-[0_0_6px] shadow-emerald-400" />
          {t("chrome.live")}
        </div>

        {authEnabled && (
          <ToolbarAction
            label={t("auth.logout")}
            icon={LogOut}
            onClick={() => onLogout?.()}
            className="ml-auto md:ml-0"
          />
        )}
      </div>

      {/* main area */}
      {isDesktop ? (
        <ResizablePanelGroup
          orientation="horizontal"
          className="flex-1"
          defaultLayout={hLayout.defaultLayout}
          onLayoutChanged={hLayout.onLayoutChanged}
        >
          <ResizablePanel id="sidebar" defaultSize="19%" minSize="14%" maxSize="28%">
            {sidebar()}
          </ResizablePanel>
          <ResizableHandle withHandle />
          <ResizablePanel id="main" defaultSize="81%">
            {mainView}
          </ResizablePanel>
        </ResizablePanelGroup>
      ) : (
        <div className="min-h-0 flex-1">{mainView}</div>
      )}

      {/* mobile: sidebar drawer */}
      <Sheet open={sidebarOpen && !isDesktop} onOpenChange={setSidebarOpen}>
        <SheetContent side="left" className="p-0" showCloseButton={false}>
          <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
            <SheetTitle className="flex select-none items-center gap-1.5 text-[15px] font-medium">
              <BrandMark className="size-4 text-primary" />
              <span>Flux<b className="text-primary">Torrent</b></span>
            </SheetTitle>
            <SheetClose className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground">
              <X className="size-4" />
              <span className="sr-only">{t("action.close")}</span>
            </SheetClose>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">{sidebar(() => setSidebarOpen(false))}</div>
        </SheetContent>
      </Sheet>

      {/* mobile: detail drawer (opens when a torrent is selected) */}
      <Sheet
        open={!isDesktop && !!sel}
        onOpenChange={(o) => {
          if (!o) setSelected(null);
        }}
      >
        <SheetContent side="bottom" className="p-0" showCloseButton={false}>
          <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b px-4">
            <SheetTitle className="truncate text-sm font-semibold">{sel ? shortName(sel.name) : ""}</SheetTitle>
            <SheetClose className="shrink-0 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground">
              <X className="size-4" />
              <span className="sr-only">{t("action.close")}</span>
            </SheetClose>
          </div>
          <div className="min-h-0 flex-1">
            <DetailPanel torrent={sel} />
          </div>
        </SheetContent>
      </Sheet>

      <StatusBar count={totals.count} down={totals.down} up={totals.up} version={version} disk={disk} />
    </div>
  );
}

// BrandMark is the wordmark icon: the favicon's "flux" speed lines (the play
// triangle is dropped here — the wordmark text carries the name).
function BrandMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden>
      <rect x="9" y="5.5" width="12" height="3" rx="1.5" opacity="0.5" />
      <rect x="3" y="10.5" width="18" height="3" rx="1.5" />
      <rect x="6" y="15.5" width="15" height="3" rx="1.5" opacity="0.65" />
    </svg>
  );
}

function ToolbarAction({
  label,
  icon: Icon,
  onClick,
  disabled,
  danger,
  className,
}: {
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  onClick: () => void;
  disabled?: boolean;
  danger?: boolean;
  className?: string;
}) {
  return (
    <Tooltip>
      {/* Wrap in a span so the tooltip still shows when the button is disabled
          (a disabled <button> emits no pointer events, so it can't be the
          hover target itself). */}
      <TooltipTrigger asChild>
        <span className={`inline-flex ${className ?? ""}`}>
          <Button
            variant="ghost"
            size="icon"
            className={`size-8 ${danger ? "hover:text-destructive" : ""}`}
            disabled={disabled}
            onClick={onClick}
          >
            <Icon className="size-4" />
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
