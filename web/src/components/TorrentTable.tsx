import { Waves, Recycle, Play, Link2, Pause, Trash2, Eye, Lock, MoreVertical } from "lucide-react";
import { type TorrentInfo } from "@/api";
import { fmtMins, fmtSize, fmtSpeed, friendlyAgent, hostOf, shortName, stateView } from "@/util";
import { useI18n } from "@/i18n";
import { cn } from "@/lib/utils";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const TONE: Record<string, string> = {
  good: "bg-emerald-400 shadow-emerald-400",
  busy: "bg-sky-400 shadow-sky-400",
  warn: "bg-amber-400 shadow-amber-400",
  idle: "bg-zinc-500 shadow-zinc-500",
};

// seedLeft describes how long until a torrent stops being shared. The seed
// clock only starts once the download reaches 100%, and a torrent may be bound
// by a time target, a ratio target, both, or neither.
//   null → render "—" (no time-based sharing obligation)
type SeedLeft = { key: string } | { value: string };
function seedLeft(x: TorrentInfo): SeedLeft | null {
  if (x.kind !== "seeding") return null;
  if (x.seedTargetMinutes <= 0) {
    return x.seedTargetRatio > 0 ? { key: "seed.byRatio" } : null;
  }
  if (x.stats.progress < 0.999) return { key: "seed.pending" };
  const remaining = Math.max(0, x.seedTargetMinutes - x.seedElapsedMin);
  if (remaining === 0) return { key: "seed.done" };
  return { value: fmtMins(remaining) };
}

interface RowActions {
  onPlay: (t: TorrentInfo) => void;
  onCopy: (t: TorrentInfo) => void;
  onDrop: (t: TorrentInfo) => void;
  onDelete: (t: TorrentInfo, withFiles?: boolean) => void;
}

export function TorrentTable({
  torrents,
  selected,
  onSelect,
  compact = false,
  ...actions
}: {
  torrents: TorrentInfo[];
  selected: string | null;
  onSelect: (hash: string) => void;
  compact?: boolean;
} & RowActions) {
  const { t } = useI18n();

  if (torrents.length === 0) {
    return (
      <div className="flex h-full items-center justify-center px-4 text-center text-sm text-muted-foreground">
        {t("empty.subtitle")}
      </div>
    );
  }

  if (compact) {
    return (
      <ScrollArea className="h-full">
        <div className="flex flex-col gap-2 p-2">
          {torrents.map((x) => (
            <TorrentCard
              key={x.hash}
              x={x}
              selected={selected === x.hash}
              onSelect={onSelect}
              {...actions}
            />
          ))}
        </div>
      </ScrollArea>
    );
  }

  return (
    <ScrollArea className="h-full">
      <Table className="text-[13px]">
        <TableHeader className="sticky top-0 z-10 bg-card">
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[26%] min-w-[200px]">{t("col.name")}</TableHead>
            <TableHead className="w-[120px]">{t("col.mode")}</TableHead>
            <TableHead className="hidden w-[150px] xl:table-cell">{t("col.progress")}</TableHead>
            <TableHead className="text-right">{t("col.size")}</TableHead>
            <TableHead className="hidden text-right sm:table-cell">{t("col.down")}</TableHead>
            <TableHead className="hidden text-right sm:table-cell">{t("col.up")}</TableHead>
            <TableHead className="hidden text-right lg:table-cell">{t("col.shared")}</TableHead>
            <TableHead className="text-right">{t("col.seedLeft")}</TableHead>
            <TableHead className="hidden text-right lg:table-cell">{t("col.peers")}</TableHead>
            <TableHead className="hidden text-right md:table-cell">{t("col.ratio")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {torrents.map((x) => {
            const sv = stateView(x.stats.state);
            const pct = Math.round(x.stats.progress * 100);
            const isSeed = x.kind === "seeding";
            const left = seedLeft(x);
            return (
              <ContextMenu key={x.hash}>
                <ContextMenuTrigger asChild>
                  <TableRow
                    data-state={selected === x.hash ? "selected" : undefined}
                    onClick={() => onSelect(x.hash)}
                    onDoubleClick={() => actions.onPlay(x)}
                    className="cursor-default"
                  >
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2">
                        <span className={cn("size-2 shrink-0 rounded-full shadow-[0_0_6px]", TONE[sv.tone])} />
                        <span className="truncate">{shortName(x.name)}</span>
                      </div>
                      <div className="flex items-center gap-1.5 pl-4 text-xs text-muted-foreground">
                        <span>{t(sv.key)}</span>
                        {x.clients.length > 0 && (
                          <span className="flex items-center gap-1 rounded bg-violet-500/15 px-1.5 py-px text-violet-300">
                            <Eye className="size-3" />
                            {friendlyAgent(x.clients[0].agent)} · {hostOf(x.clients[0].addr)}
                            {x.clients.length > 1 && ` +${x.clients.length - 1}`}
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-1">
                        <Badge
                          variant="outline"
                          className={cn(
                            "gap-1 font-normal",
                            isSeed ? "border-emerald-500/40 text-emerald-400" : "border-sky-500/40 text-sky-400"
                          )}
                        >
                          {isSeed ? <Recycle className="size-3" /> : <Waves className="size-3" />}
                          {t(isSeed ? "tag.sharing" : "tag.streaming")}
                        </Badge>
                        {x.private && (
                          <Badge variant="outline" className="gap-1 border-amber-500/40 font-normal text-amber-400" title={t("tag.private")}>
                            <Lock className="size-3" /> {t("tag.private")}
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="hidden xl:table-cell">
                      <div className="flex items-center gap-2">
                        <Progress value={pct} className="h-1.5" />
                        <span className="w-9 shrink-0 text-right text-xs tabular text-muted-foreground">{pct}%</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-right tabular text-muted-foreground">{fmtSize(x.sizeBytes)}</TableCell>
                    <TableCell className="hidden text-right tabular text-emerald-400/90 sm:table-cell">{fmtSpeed(x.stats.downKbps)}</TableCell>
                    <TableCell className="hidden text-right tabular text-sky-400/80 sm:table-cell">{fmtSpeed(x.stats.upKbps)}</TableCell>
                    <TableCell className="hidden text-right tabular text-sky-400/80 lg:table-cell">
                      {x.stats.uploadedBytes > 0 ? fmtSize(x.stats.uploadedBytes) : "—"}
                    </TableCell>
                    <TableCell className="text-right tabular text-muted-foreground">
                      {left == null ? (
                        "—"
                      ) : "value" in left ? (
                        <span className="text-emerald-400/90">{left.value}</span>
                      ) : (
                        <span className="text-xs">{t(left.key)}</span>
                      )}
                    </TableCell>
                    <TableCell className="hidden text-right tabular text-muted-foreground lg:table-cell">
                      {x.stats.seeders}/{x.stats.peers}
                    </TableCell>
                    <TableCell className="hidden text-right tabular text-muted-foreground md:table-cell">
                      {x.stats.ratio.toFixed(2)}
                    </TableCell>
                  </TableRow>
                </ContextMenuTrigger>
                <ContextMenuContent className="w-52">
                  <ContextMenuItem onClick={() => actions.onPlay(x)}>
                    <Play data-icon="inline-start" /> {t("menu.play")}
                  </ContextMenuItem>
                  <ContextMenuItem onClick={() => actions.onCopy(x)}>
                    <Link2 data-icon="inline-start" /> {t("menu.copy")}
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem onClick={() => actions.onDrop(x)}>
                    <Pause data-icon="inline-start" /> {t("menu.drop")}
                  </ContextMenuItem>
                  <ContextMenuItem variant="destructive" onClick={() => actions.onDelete(x)}>
                    <Trash2 data-icon="inline-start" /> {t("menu.delete")}
                  </ContextMenuItem>
                  <ContextMenuItem variant="destructive" onClick={() => actions.onDelete(x, true)}>
                    <Trash2 data-icon="inline-start" /> {t("menu.deleteFiles")}
                  </ContextMenuItem>
                </ContextMenuContent>
              </ContextMenu>
            );
          })}
        </TableBody>
      </Table>
    </ScrollArea>
  );
}

// TorrentCard is the mobile/compact row: a tappable card with the essentials
// plus an actions menu (the table's context menu isn't discoverable on touch).
function TorrentCard({
  x,
  selected,
  onSelect,
  onPlay,
  onCopy,
  onDrop,
  onDelete,
}: { x: TorrentInfo; selected: boolean; onSelect: (hash: string) => void } & RowActions) {
  const { t } = useI18n();
  const sv = stateView(x.stats.state);
  const pct = Math.round(x.stats.progress * 100);
  const isSeed = x.kind === "seeding";
  const left = seedLeft(x);

  return (
    <div
      onClick={() => onSelect(x.hash)}
      data-state={selected ? "selected" : undefined}
      className="rounded-lg border bg-card p-3 transition-colors data-[state=selected]:border-primary/50 data-[state=selected]:bg-accent/40"
    >
      <div className="flex items-start gap-2">
        <span className={cn("mt-1.5 size-2 shrink-0 rounded-full shadow-[0_0_6px]", TONE[sv.tone])} />
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{shortName(x.name)}</div>
          <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <span>{t(sv.key)}</span>
            <Badge
              variant="outline"
              className={cn(
                "gap-1 px-1.5 py-0 font-normal",
                isSeed ? "border-emerald-500/40 text-emerald-400" : "border-sky-500/40 text-sky-400"
              )}
            >
              {isSeed ? <Recycle className="size-3" /> : <Waves className="size-3" />}
              {t(isSeed ? "tag.sharing" : "tag.streaming")}
            </Badge>
            {x.private && (
              <Badge variant="outline" className="gap-1 px-1.5 py-0 font-normal border-amber-500/40 text-amber-400">
                <Lock className="size-3" /> {t("tag.private")}
              </Badge>
            )}
          </div>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger
            onClick={(e) => e.stopPropagation()}
            className="-mr-1 -mt-1 rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            <MoreVertical className="size-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48" onClick={(e) => e.stopPropagation()}>
            <DropdownMenuItem onClick={() => onPlay(x)}>
              <Play data-icon="inline-start" /> {t("menu.play")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => onCopy(x)}>
              <Link2 data-icon="inline-start" /> {t("menu.copy")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => onDrop(x)}>
              <Pause data-icon="inline-start" /> {t("menu.drop")}
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onClick={() => onDelete(x)}>
              <Trash2 data-icon="inline-start" /> {t("menu.delete")}
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onClick={() => onDelete(x, true)}>
              <Trash2 data-icon="inline-start" /> {t("menu.deleteFiles")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="mt-2 flex items-center gap-2">
        <Progress value={pct} className="h-1.5" />
        <span className="w-9 shrink-0 text-right text-xs tabular text-muted-foreground">{pct}%</span>
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs tabular text-muted-foreground">
        <span>{fmtSize(x.sizeBytes)}</span>
        <span className="text-emerald-400/90">↓ {fmtSpeed(x.stats.downKbps)}</span>
        <span className="text-sky-400/80">↑ {fmtSpeed(x.stats.upKbps)}</span>
        {x.stats.uploadedBytes > 0 && (
          <span className="text-sky-400/80">
            {t("col.shared")}: {fmtSize(x.stats.uploadedBytes)}
          </span>
        )}
        {left != null && (
          <span>
            {t("col.seedLeft")}:{" "}
            {"value" in left ? <span className="text-emerald-400/90">{left.value}</span> : t(left.key)}
          </span>
        )}
        <span>
          {x.stats.seeders}/{x.stats.peers} · {x.stats.ratio.toFixed(2)}
        </span>
      </div>
    </div>
  );
}
