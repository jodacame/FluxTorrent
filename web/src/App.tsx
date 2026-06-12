import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  Plus,
  Play,
  Link2,
  Pause,
  Trash2,
  Loader2,
} from "lucide-react";
import { api, connectEvents, type TorrentInfo } from "./api";
import { shortName } from "./util";
import { useI18n } from "./i18n";
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
import { Sidebar, type Filter } from "@/components/Sidebar";
import { TorrentTable } from "@/components/TorrentTable";
import { DetailPanel } from "@/components/DetailPanel";
import { RulesEditor } from "@/components/RulesEditor";
import { SettingsView } from "@/components/SettingsView";
import { StatusBar } from "@/components/StatusBar";

export type View = "library" | "rules" | "settings";

export default function App() {
  const { t } = useI18n();
  const [torrents, setTorrents] = useState<TorrentInfo[]>([]);
  const [view, setView] = useState<View>("library");
  const [filter, setFilter] = useState<Filter>("all");
  const [selected, setSelected] = useState<string | null>(null);
  const [link, setLink] = useState("");
  const [adding, setAdding] = useState(false);
  const [version, setVersion] = useState("");

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
  const copy = (x: TorrentInfo) => {
    const f = x.files.find((y) => y.playable) ?? x.files[0];
    if (!f) return;
    navigator.clipboard.writeText(api.streamUrl(x.hash, f.index));
    toast.success(t("toast.copied"));
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

  return (
    <div className="flex h-screen flex-col bg-background text-foreground">
      {/* toolbar */}
      <div className="flex shrink-0 items-center gap-2 border-b bg-card/60 px-3 py-2">
        <div className="flex select-none items-center gap-1.5 pr-1 text-[15px]">
          <span className="text-primary">◖</span>
          <span className="font-medium">Flux<b className="text-primary">Torrent</b></span>
        </div>
        <Separator orientation="vertical" className="mx-1 h-6" />

        <div className="flex w-full max-w-md items-center gap-2">
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
          <Button size="sm" onClick={add} disabled={adding || !link.trim()} className="h-8">
            {adding ? <Loader2 className="animate-spin" data-icon="inline-start" /> : <Plus data-icon="inline-start" />}
            {t("add.button")}
          </Button>
        </div>

        <Separator orientation="vertical" className="mx-1 h-6" />

        <ToolbarAction label={t("menu.play")} disabled={!sel} onClick={() => sel && play(sel)} icon={Play} />
        <ToolbarAction label={t("menu.copy")} disabled={!sel} onClick={() => sel && copy(sel)} icon={Link2} />
        <ToolbarAction label={t("menu.drop")} disabled={!sel} onClick={() => sel && api.drop(sel.hash)} icon={Pause} />
        <ToolbarAction label={t("menu.delete")} disabled={!sel} onClick={() => sel && remove(sel)} icon={Trash2} danger />

        <div className="ml-auto flex items-center gap-1.5 pr-1 text-xs text-muted-foreground">
          <span className="size-1.5 rounded-full bg-emerald-400 shadow-[0_0_6px] shadow-emerald-400" />
          {t("chrome.live")}
        </div>
      </div>

      {/* main split */}
      <ResizablePanelGroup
        orientation="horizontal"
        className="flex-1"
        defaultLayout={hLayout.defaultLayout}
        onLayoutChanged={hLayout.onLayoutChanged}
      >
        <ResizablePanel id="sidebar" defaultSize="19%" minSize="14%" maxSize="28%">
          <Sidebar
            view={view}
            onView={setView}
            filter={filter}
            onFilter={setFilter}
            counts={{
              all: torrents.length,
              streaming: torrents.filter((x) => x.kind === "stream").length,
              sharing: torrents.filter((x) => x.kind === "seeding").length,
              downloading: torrents.filter((x) => x.stats.progress < 0.999).length,
              completed: torrents.filter((x) => x.stats.progress >= 0.999).length,
            }}
          />
        </ResizablePanel>
        <ResizableHandle withHandle />
        <ResizablePanel id="main" defaultSize="81%">
          {view === "rules" ? (
            <RulesEditor />
          ) : view === "settings" ? (
            <SettingsView />
          ) : (
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
                <DetailPanel torrent={sel} onCopy={copy} />
              </ResizablePanel>
            </ResizablePanelGroup>
          )}
        </ResizablePanel>
      </ResizablePanelGroup>

      <StatusBar count={totals.count} down={totals.down} up={totals.up} version={version} />
    </div>
  );
}

function ToolbarAction({
  label,
  icon: Icon,
  onClick,
  disabled,
  danger,
}: {
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  onClick: () => void;
  disabled?: boolean;
  danger?: boolean;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className={`size-8 ${danger ? "hover:text-destructive" : ""}`}
          disabled={disabled}
          onClick={onClick}
        >
          <Icon className="size-4" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
