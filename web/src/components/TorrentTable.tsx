import { Waves, Recycle, Play, Link2, Pause, Trash2, Eye, Lock } from "lucide-react";
import { type TorrentInfo } from "@/api";
import { fmtSize, fmtSpeed, friendlyAgent, hostOf, shortName, stateView } from "@/util";
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

const TONE: Record<string, string> = {
  good: "bg-emerald-400 shadow-emerald-400",
  busy: "bg-sky-400 shadow-sky-400",
  warn: "bg-amber-400 shadow-amber-400",
  idle: "bg-zinc-500 shadow-zinc-500",
};

export function TorrentTable({
  torrents,
  selected,
  onSelect,
  onPlay,
  onCopy,
  onDrop,
  onDelete,
}: {
  torrents: TorrentInfo[];
  selected: string | null;
  onSelect: (hash: string) => void;
  onPlay: (t: TorrentInfo) => void;
  onCopy: (t: TorrentInfo) => void;
  onDrop: (t: TorrentInfo) => void;
  onDelete: (t: TorrentInfo, withFiles?: boolean) => void;
}) {
  const { t } = useI18n();

  if (torrents.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        {t("empty.subtitle")}
      </div>
    );
  }

  return (
    <ScrollArea className="h-full">
      <Table className="text-[13px]">
        <TableHeader className="sticky top-0 z-10 bg-card">
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[34%]">{t("col.name")}</TableHead>
            <TableHead className="w-[120px]">{t("col.mode")}</TableHead>
            <TableHead className="w-[180px]">{t("col.progress")}</TableHead>
            <TableHead className="text-right">{t("col.size")}</TableHead>
            <TableHead className="text-right">{t("col.down")}</TableHead>
            <TableHead className="text-right">{t("col.up")}</TableHead>
            <TableHead className="text-right">{t("col.peers")}</TableHead>
            <TableHead className="text-right">{t("col.ratio")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {torrents.map((x) => {
            const sv = stateView(x.stats.state);
            const pct = Math.round(x.stats.progress * 100);
            const isSeed = x.kind === "seeding";
            return (
              <ContextMenu key={x.hash}>
                <ContextMenuTrigger asChild>
                  <TableRow
                    data-state={selected === x.hash ? "selected" : undefined}
                    onClick={() => onSelect(x.hash)}
                    onDoubleClick={() => onPlay(x)}
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
                      <div className="flex items-center gap-1">
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
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Progress value={pct} className="h-1.5" />
                        <span className="w-9 shrink-0 text-right text-xs tabular text-muted-foreground">{pct}%</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-right tabular text-muted-foreground">{fmtSize(x.sizeBytes)}</TableCell>
                    <TableCell className="text-right tabular text-emerald-400/90">{fmtSpeed(x.stats.downKbps)}</TableCell>
                    <TableCell className="text-right tabular text-sky-400/80">{fmtSpeed(x.stats.upKbps)}</TableCell>
                    <TableCell className="text-right tabular text-muted-foreground">
                      {x.stats.seeders}/{x.stats.peers}
                    </TableCell>
                    <TableCell className="text-right tabular text-muted-foreground">
                      {x.stats.ratio.toFixed(2)}
                    </TableCell>
                  </TableRow>
                </ContextMenuTrigger>
                <ContextMenuContent className="w-52">
                  <ContextMenuItem onClick={() => onPlay(x)}>
                    <Play data-icon="inline-start" /> {t("menu.play")}
                  </ContextMenuItem>
                  <ContextMenuItem onClick={() => onCopy(x)}>
                    <Link2 data-icon="inline-start" /> {t("menu.copy")}
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem onClick={() => onDrop(x)}>
                    <Pause data-icon="inline-start" /> {t("menu.drop")}
                  </ContextMenuItem>
                  <ContextMenuItem variant="destructive" onClick={() => onDelete(x)}>
                    <Trash2 data-icon="inline-start" /> {t("menu.delete")}
                  </ContextMenuItem>
                  <ContextMenuItem variant="destructive" onClick={() => onDelete(x, true)}>
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
