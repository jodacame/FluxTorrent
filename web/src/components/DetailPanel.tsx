import { Copy, Eye, Radio } from "lucide-react";
import { type TorrentInfo } from "@/api";
import { fmtSize, fmtSpeed, friendlyAgent, hostOf } from "@/util";
import { api } from "@/api";
import { useI18n } from "@/i18n";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

export function DetailPanel({
  torrent,
  onCopy,
}: {
  torrent: TorrentInfo | null;
  onCopy: (t: TorrentInfo) => void;
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
        <TabsList className="h-9 w-full justify-start rounded-none border-b bg-transparent px-2">
          <TabsTrigger value="general">{t("detail.general")}</TabsTrigger>
          <TabsTrigger value="files">{t("detail.files")} ({x.files.length})</TabsTrigger>
          <TabsTrigger value="sources">{t("detail.sources")}</TabsTrigger>
          <TabsTrigger value="client">{t("detail.client")} ({x.clients.length})</TabsTrigger>
        </TabsList>

        <ScrollArea className="flex-1">
          <TabsContent value="general" className="m-0 p-4">
            <div className="grid grid-cols-[140px_1fr] gap-x-4 gap-y-2 text-[13px]">
              <Field label={t("col.name")} value={x.name} />
              <Field label={t("detail.hash")} value={<code className="text-xs">{x.hash}</code>} />
              <Field label={t("col.size")} value={fmtSize(x.sizeBytes)} />
              <Field
                label={t("detail.mode")}
                value={
                  <Badge variant="outline" className={isSeed ? "border-emerald-500/40 text-emerald-400" : "border-sky-500/40 text-sky-400"}>
                    {t(isSeed ? "tag.sharing" : "tag.streaming")} · {t(x.storageMode === "disk" ? "tag.modeDisk" : "tag.modeRam")}
                  </Badge>
                }
              />
              {isSeed && (
                <Field
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
              <Field label={t("detail.peers")} value={`${x.stats.seeders} / ${x.stats.peers}`} />
            </div>
          </TabsContent>

          <TabsContent value="files" className="m-0 p-2">
            <div className="flex flex-col gap-0.5">
              {x.files.map((f) => (
                <div key={f.index} className="flex items-center gap-2 rounded px-2 py-1.5 text-[13px] hover:bg-accent/50">
                  <span className={`size-1.5 rounded-full ${f.playable ? "bg-emerald-400" : "bg-muted-foreground/40"}`} />
                  <span className="flex-1 truncate">{f.path}</span>
                  <span className="tabular text-xs text-muted-foreground">{fmtSize(f.sizeBytes)}</span>
                  {f.playable && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-6"
                      onClick={() => {
                        navigator.clipboard.writeText(api.streamUrl(x.hash, f.index));
                        toast.success(t("toast.copied"));
                      }}
                    >
                      <Copy className="size-3.5" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          </TabsContent>

          <TabsContent value="sources" className="m-0 p-4">
            <div className="flex items-center gap-6 text-[13px]">
              <div className="flex items-center gap-2">
                <Radio className="size-4 text-emerald-400" />
                <span className="tabular text-lg">{x.stats.seeders}</span>
                <span className="text-muted-foreground">{t("detail.seeders")}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="tabular text-lg">{x.stats.peers}</span>
                <span className="text-muted-foreground">{t("detail.peers")}</span>
              </div>
              <div className="ml-auto tabular text-emerald-400">↓ {fmtSpeed(x.stats.downKbps)}</div>
              <div className="tabular text-sky-400">↑ {fmtSpeed(x.stats.upKbps)}</div>
            </div>
          </TabsContent>

          <TabsContent value="client" className="m-0 p-2">
            {x.clients.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">{t("detail.noClients")}</div>
            ) : (
              <div className="flex flex-col gap-1">
                {x.clients.map((c, i) => (
                  <div key={i} className="flex items-center gap-3 rounded-md border bg-card px-3 py-2 text-[13px]">
                    <Eye className="size-4 text-violet-400" />
                    <span className="font-medium">{friendlyAgent(c.agent)}</span>
                    <code className="text-xs text-muted-foreground">{hostOf(c.addr)}</code>
                    <span className="ml-auto truncate text-xs text-muted-foreground">{c.file}</span>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>
        </ScrollArea>
      </Tabs>

      <div className="flex items-center gap-2 border-t px-3 py-1.5">
        <Button variant="ghost" size="sm" className="h-7" onClick={() => onCopy(x)}>
          <Copy data-icon="inline-start" /> {t("detail.copyStream")}
        </Button>
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <>
      <div className="text-muted-foreground">{label}</div>
      <div className="truncate">{value}</div>
    </>
  );
}
