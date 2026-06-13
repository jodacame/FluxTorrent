import { Copy, Eye } from "lucide-react";
import { type TorrentInfo } from "@/api";
import { copyText, fmtSize, fmtSpeed, friendlyAgent, hostOf } from "@/util";
import { api } from "@/api";
import { useI18n } from "@/i18n";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { toast } from "sonner";

export function DetailPanel({
  torrent,
}: {
  torrent: TorrentInfo | null;
}) {
  const { t } = useI18n();

  if (!torrent) {
    return (
      <div className="flex h-full items-center justify-center bg-card/40 text-sm text-muted-foreground">
        {t("detail.none")}
      </div>
    );
  }
  const x = torrent;
  const isSeed = x.kind === "seeding";

  return (
    <div className="flex h-full flex-col bg-card/40">
      <Tabs defaultValue="general" className="flex h-full flex-col gap-0">
        <TabsList className="h-9 w-full shrink-0 justify-start gap-1 overflow-x-auto rounded-none border-b bg-transparent px-2 [&_[data-slot=tabs-trigger]]:flex-none">
          <TabsTrigger value="general">{t("detail.general")}</TabsTrigger>
          <TabsTrigger value="files">{t("detail.files")} ({x.files.length})</TabsTrigger>
          <TabsTrigger value="sources">{t("detail.sources")} ({x.peers.length})</TabsTrigger>
          <TabsTrigger value="trackers">{t("detail.trackers")} ({x.trackers.length})</TabsTrigger>
          <TabsTrigger value="client">{t("detail.client")} ({x.clients.length})</TabsTrigger>
        </TabsList>

        <ScrollArea className="flex-1">
          <TabsContent value="general" className="m-0">
            <Table className="text-[13px]">
              <TableBody>
                <KV label={t("col.name")} value={<span className="break-all">{x.name}</span>} />
                <KV label={t("detail.hash")} value={<code className="text-xs break-all">{x.hash}</code>} />
                <KV label={t("col.size")} value={fmtSize(x.sizeBytes)} />
                <KV
                  label={t("detail.mode")}
                  value={
                    <Badge variant="outline" className={isSeed ? "border-emerald-500/40 text-emerald-400" : "border-sky-500/40 text-sky-400"}>
                      {t(isSeed ? "tag.sharing" : "tag.streaming")} · {t(x.storageMode === "disk" ? "tag.modeDisk" : "tag.modeRam")}
                    </Badge>
                  }
                />
                {x.stats.uploadedBytes > 0 && <KV label={t("col.shared")} value={fmtSize(x.stats.uploadedBytes)} />}
                {isSeed && (
                  <KV
                    label={t("detail.seedTarget")}
                    value={
                      <span className="tabular">
                        {x.seedTargetRatio > 0 && t("tag.ratio", { cur: x.stats.ratio.toFixed(1), target: x.seedTargetRatio.toFixed(1) })}
                        {x.seedTargetRatio > 0 && x.seedTargetMinutes > 0 && " · "}
                        {x.seedTargetMinutes > 0 && t("tag.shareTime", { cur: x.seedElapsedMin, target: x.seedTargetMinutes })}
                      </span>
                    }
                  />
                )}
                <KV label={t("detail.peers")} value={`${x.stats.seeders} / ${x.stats.peers}`} />
              </TableBody>
            </Table>
          </TabsContent>

          <TabsContent value="files" className="m-0">
            <Table className="text-[13px]">
              <TableHeader>
                <TableRow>
                  <TableHead className="min-w-[200px]">{t("col.name")}</TableHead>
                  <TableHead className="text-right">{t("col.size")}</TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {x.files.map((f) => (
                  <TableRow key={f.index}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span className={`size-1.5 shrink-0 rounded-full ${f.playable ? "bg-emerald-400" : "bg-muted-foreground/40"}`} />
                        <span className="truncate">{f.path}</span>
                      </div>
                    </TableCell>
                    <TableCell className="whitespace-nowrap text-right tabular text-muted-foreground">{fmtSize(f.sizeBytes)}</TableCell>
                    <TableCell className="text-right">
                      {f.playable && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-6"
                          onClick={async () => {
                            const ok = await copyText(api.streamUrl(x.hash, f.index));
                            if (ok) toast.success(t("toast.copied"));
                            else toast.error(t("toast.copyFailed"));
                          }}
                        >
                          <Copy className="size-3.5" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TabsContent>

          <TabsContent value="sources" className="m-0">
            {/* summary line */}
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b px-3 py-2 text-xs tabular text-muted-foreground">
              <span>
                <span className="text-emerald-400">{x.stats.seeders}</span> {t("detail.seeders")}
              </span>
              <span>
                <span className="text-foreground">{x.stats.peers}</span> {t("detail.peers")}
              </span>
              <span className="ml-auto text-emerald-400">↓ {fmtSpeed(x.stats.downKbps)}</span>
              <span className="text-sky-400">↑ {fmtSpeed(x.stats.upKbps)}</span>
            </div>
            {x.peers.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">{t("detail.noPeers")}</div>
            ) : (
              <Table className="text-[13px]">
                <TableHeader>
                  <TableRow>
                    <TableHead className="min-w-[140px]">{t("detail.address")}</TableHead>
                    <TableHead>{t("detail.peerClient")}</TableHead>
                    <TableHead>{t("detail.peerType")}</TableHead>
                    <TableHead className="text-right">{t("status.down")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {x.peers.map((p, i) => (
                    <TableRow key={i}>
                      <TableCell className="whitespace-nowrap tabular">{p.addr}</TableCell>
                      <TableCell className="max-w-[160px] truncate text-muted-foreground">{p.client || "—"}</TableCell>
                      <TableCell>
                        <span className={p.seeder ? "text-emerald-400" : "text-sky-400"}>
                          {t(p.seeder ? "detail.seeder" : "detail.leecher")}
                        </span>
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-right tabular text-emerald-400">
                        {p.downKbps > 0 ? fmtSpeed(p.downKbps) : "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </TabsContent>

          <TabsContent value="trackers" className="m-0">
            {x.trackers.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">{t("detail.noTrackers")}</div>
            ) : (
              <Table className="text-[13px]">
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-8">#</TableHead>
                    <TableHead>{t("detail.tracker")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {x.trackers.map((tr, i) => (
                    <TableRow key={i}>
                      <TableCell className="tabular text-muted-foreground">{i + 1}</TableCell>
                      <TableCell className="break-all">{tr}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </TabsContent>

          <TabsContent value="client" className="m-0">
            {x.clients.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">{t("detail.noClients")}</div>
            ) : (
              <Table className="text-[13px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("detail.player")}</TableHead>
                    <TableHead>{t("detail.address")}</TableHead>
                    <TableHead className="min-w-[160px]">{t("detail.file")}</TableHead>
                    <TableHead className="text-right">{t("detail.send")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {x.clients.map((c, i) => (
                    <TableRow key={i}>
                      <TableCell>
                        <div className="flex items-center gap-1.5 whitespace-nowrap font-medium">
                          <Eye className="size-3.5 shrink-0 text-violet-400" />
                          {friendlyAgent(c.agent)}
                        </div>
                      </TableCell>
                      <TableCell>
                        <code className="text-xs text-muted-foreground">{hostOf(c.addr)}</code>
                      </TableCell>
                      <TableCell className="max-w-[220px] truncate text-muted-foreground">{c.file}</TableCell>
                      <TableCell className="whitespace-nowrap text-right tabular text-sky-400" title={t("detail.sendSpeed")}>
                        {fmtSpeed(c.sendKbps)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </TabsContent>
        </ScrollArea>
      </Tabs>
    </div>
  );
}

// KV is a key/value row for the General and Sources tables.
function KV({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <TableRow>
      <TableCell className="w-[140px] align-top whitespace-nowrap font-medium text-muted-foreground">{label}</TableCell>
      <TableCell className="align-top">{value}</TableCell>
    </TableRow>
  );
}
