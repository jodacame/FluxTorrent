import { useEffect, useState } from "react";
import { Save, HardDrive, Trash2, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { api, type Settings } from "@/api";
import { fmtSize } from "@/util";
import { useI18n } from "@/i18n";
import { cn } from "@/lib/utils";
import { Progress } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { HelpDialog } from "@/components/HelpDialog";

export function SettingsView() {
  const { t, lang, setLang, locales } = useI18n();
  const [s, setS] = useState<Settings | null>(null);
  const [saving, setSaving] = useState(false);
  const [advanced, setAdvanced] = useState(false);

  useEffect(() => {
    api.getSettings().then(setS).catch(() => {});
  }, []);

  if (!s) return <div className="p-6 text-sm text-muted-foreground">…</div>;

  const save = async () => {
    setSaving(true);
    try {
      await api.putSettings(s);
      toast.success(t("toast.saved"));
    } catch (e) {
      toast.error(t("toast.saveError", { msg: (e as Error).message }));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex shrink-0 items-center gap-1 border-b bg-card/60 px-4 py-3">
        <h2 className="text-sm font-semibold">{t("nav.settings")}</h2>
        <HelpDialog />
        <Button size="sm" className="ml-auto" onClick={save} disabled={saving}>
          <Save data-icon="inline-start" /> {saving ? t("settings.saving") : t("settings.save")}
        </Button>
      </div>

      <ScrollArea className="min-h-0 flex-1">
        <div className="mx-auto flex max-w-2xl flex-col gap-6 p-4 sm:p-6">
          <Group title={t("settings.storage")}>
            <div className="grid grid-cols-2 gap-3">
              <ModeCard
                active={s.cache.mode === "ram"}
                title={t("settings.ram")}
                help={t("settings.ramHelp")}
                onClick={() => setS({ ...s, cache: { ...s.cache, mode: "ram" } })}
              />
              <ModeCard
                active={s.cache.mode === "disk"}
                title={t("settings.disk")}
                help={t("settings.diskHelp")}
                onClick={() => setS({ ...s, cache: { ...s.cache, mode: "disk" } })}
              />
            </div>
            <NumField label={t("settings.cacheSize")} help={t("settings.cacheSizeHelp")} value={s.cache.sizeMB}
              onChange={(v) => setS({ ...s, cache: { ...s.cache, sizeMB: v } })} />
            <NumField label={t("settings.readahead")} help={t("settings.readaheadHelp")} value={s.cache.readaheadMB}
              onChange={(v) => setS({ ...s, cache: { ...s.cache, readaheadMB: v } })} />
            <TextField label={t("settings.path")} help={t("settings.pathHelp")} value={s.cache.path}
              onChange={(v) => setS({ ...s, cache: { ...s.cache, path: v } })} />
            <DiskUsage />
          </Group>

          <Group title={t("settings.cleanup")}>
            <NumField label={t("settings.diskMaxGB")} help={t("settings.diskMaxGBHelp")} value={s.disk.maxGB}
              onChange={(v) => setS({ ...s, disk: { ...s.disk, maxGB: v } })} />
            <NumField label={t("settings.graceMinutes")} help={t("settings.graceMinutesHelp")} value={s.disk.graceMinutes}
              onChange={(v) => setS({ ...s, disk: { ...s.disk, graceMinutes: v } })} />
            <SwitchField label={t("settings.deleteAfterSeed")} help={t("settings.deleteAfterSeedHelp")} value={s.disk.deleteAfterSeed}
              onChange={(v) => setS({ ...s, disk: { ...s.disk, deleteAfterSeed: v } })} />
            <SwitchField label={t("settings.deleteAfterPlayback")} help={t("settings.deleteAfterPlaybackHelp")} value={s.disk.deleteAfterPlayback}
              onChange={(v) => setS({ ...s, disk: { ...s.disk, deleteAfterPlayback: v } })} />
            <OrphanCleanup />
          </Group>

          <Group title={t("settings.language")}>
            <div className="flex items-center justify-between gap-4">
              <div>
                <Label className="text-sm">{t("settings.language")}</Label>
                <p className="text-xs text-muted-foreground">{t("settings.languageHelp")}</p>
              </div>
              <Select value={lang} onValueChange={setLang}>
                <SelectTrigger className="h-8 w-40"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {locales.map((l) => (
                    <SelectItem key={l.code} value={l.code}>{l.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </Group>

          <Group title={t("settings.network")}>
            <NumField label={t("settings.downLimit")} help={t("settings.downLimitHelp")} value={s.net.downKbps}
              onChange={(v) => setS({ ...s, net: { ...s.net, downKbps: v } })} />
            <NumField label={t("settings.upLimit")} help={t("settings.upLimitHelp")} value={s.net.upKbps}
              onChange={(v) => setS({ ...s, net: { ...s.net, upKbps: v } })} />
          </Group>

          <Group title={t("settings.seeding")}>
            <SwitchField label={t("settings.seedEnabled")} help={t("settings.seedEnabledHelp")} value={s.seed.enabled}
              onChange={(v) => setS({ ...s, seed: { ...s.seed, enabled: v } })} />
            <SwitchField label={t("settings.dropAfter")} help={t("settings.dropAfterHelp")} value={s.seed.dropAfterPlayback}
              onChange={(v) => setS({ ...s, seed: { ...s.seed, dropAfterPlayback: v } })} />
          </Group>

          <Group title={t("settings.privateTrackers")}>
            <SwitchField label={t("settings.privateAuto")} help={t("settings.privateAutoHelp")} value={s.seed.privateAuto}
              onChange={(v) => setS({ ...s, seed: { ...s.seed, privateAuto: v } })} />
            {s.seed.privateAuto && (
              <>
                <NumField label={t("settings.privateRatio")} help={t("settings.privateRatioHelp")} value={s.seed.privateMaxRatio}
                  onChange={(v) => setS({ ...s, seed: { ...s.seed, privateMaxRatio: v } })} />
                <NumField label={t("settings.privateHours")} help={t("settings.privateHoursHelp")} value={Math.round(s.seed.privateMaxMinutes / 60)}
                  onChange={(v) => setS({ ...s, seed: { ...s.seed, privateMaxMinutes: v * 60 } })} />
              </>
            )}
          </Group>

          <Group title={t("settings.advanced")}>
            <NumField label={t("settings.noPeers")} help={t("settings.noPeersHelp")} value={s.noPeersTimeoutSec}
              onChange={(v) => setS({ ...s, noPeersTimeoutSec: v })} />
            <NumField label={t("settings.maxActive")} help={t("settings.maxActiveHelp")} value={s.limits.maxActiveTorrents}
              onChange={(v) => setS({ ...s, limits: { maxActiveTorrents: v } })} />
            <SwitchField label={t("settings.rejectCompressed")} help={t("settings.rejectCompressedHelp")} value={s.compressed.reject}
              onChange={(v) => setS({ ...s, compressed: { reject: v } })} />
          </Group>

          <Group title={t("settings.advancedMode")}>
            <SwitchField label={t("settings.advancedMode")} help={t("settings.advancedModeHelp")} value={advanced}
              onChange={setAdvanced} />
            {advanced && (
              <>
                <Separator className="my-1" />
                <SwitchField label={t("settings.dht")} help={t("settings.dhtHelp")} value={s.net.dht}
                  onChange={(v) => setS({ ...s, net: { ...s.net, dht: v } })} />
                <NumField label={t("settings.maxConns")} help={t("settings.maxConnsHelp")} value={s.net.maxConns}
                  onChange={(v) => setS({ ...s, net: { ...s.net, maxConns: v } })} />
                <SwitchField label={t("settings.encrypt")} help={t("settings.encryptHelp")} value={s.net.encryptHeaders}
                  onChange={(v) => setS({ ...s, net: { ...s.net, encryptHeaders: v } })} />
                <SwitchField label={t("settings.ipv6")} help={t("settings.ipv6Help")} value={s.net.ipv6}
                  onChange={(v) => setS({ ...s, net: { ...s.net, ipv6: v } })} />
                <SwitchField label={t("settings.utp")} help={t("settings.utpHelp")} value={s.net.utp}
                  onChange={(v) => setS({ ...s, net: { ...s.net, utp: v } })} />
                <NumField label={t("settings.disconnectTimeout")} help={t("settings.disconnectTimeoutHelp")} value={s.net.disconnectTimeoutSec}
                  onChange={(v) => setS({ ...s, net: { ...s.net, disconnectTimeoutSec: v } })} />
              </>
            )}
          </Group>
        </div>
      </ScrollArea>
    </div>
  );
}

function DiskUsage() {
  const { t } = useI18n();
  const [d, setD] = useState<Awaited<ReturnType<typeof api.disk>> | null>(null);

  useEffect(() => {
    api.disk().then(setD).catch(() => {});
  }, []);

  if (!d) return null;
  if (!d.available || !d.totalBytes) {
    return <div className="text-xs text-muted-foreground">{t("settings.diskUnavailable")}: {d.path}</div>;
  }
  const pct = Math.round((d.usedBytes! / d.totalBytes) * 100);
  return (
    <div className="rounded-md border bg-card/40 px-3 py-2.5">
      <div className="mb-2 flex items-center gap-2 text-[13px]">
        <HardDrive className="size-4 text-muted-foreground" />
        <code className="text-xs text-muted-foreground">{d.path}</code>
        <span className="ml-auto tabular text-xs text-muted-foreground">
          {fmtSize(d.usedBytes!)} {t("settings.diskUsed")} · {fmtSize(d.freeBytes!)} {t("settings.diskFree")} · {fmtSize(d.totalBytes)} {t("settings.diskOf")}
        </span>
      </div>
      <Progress value={pct} className={cn("h-2", pct > 90 && "[&>*]:bg-destructive")} />
    </div>
  );
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border bg-card p-4">
      <h3 className="mb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{title}</h3>
      <Separator className="mb-3" />
      <div className="flex flex-col gap-3">{children}</div>
    </section>
  );
}

function ModeCard({ active, title, help, onClick }: { active: boolean; title: string; help: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "rounded-md border p-3 text-left transition-colors",
        active ? "border-primary bg-primary/10" : "hover:bg-accent/50"
      )}
    >
      <div className="text-sm font-medium">{title}</div>
      <div className="text-xs text-muted-foreground">{help}</div>
    </button>
  );
}

function NumField({ label, help, value, onChange }: { label: string; help: string; value: number; onChange: (v: number) => void }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <Label className="text-sm">{label}</Label>
        <p className="text-xs text-muted-foreground">{help}</p>
      </div>
      <Input type="number" value={value} onChange={(e) => onChange(Number(e.target.value))} className="h-8 w-28 text-right tabular" />
    </div>
  );
}

function TextField({ label, help, value, onChange }: { label: string; help: string; value: string; onChange: (v: string) => void }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <Label className="text-sm">{label}</Label>
        <p className="text-xs text-muted-foreground">{help}</p>
      </div>
      <Input value={value} onChange={(e) => onChange(e.target.value)} className="h-8 w-56 font-mono text-xs" />
    </div>
  );
}

function SwitchField({ label, help, value, onChange }: { label: string; help: string; value: boolean; onChange: (v: boolean) => void }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <Label className="text-sm">{label}</Label>
        <p className="text-xs text-muted-foreground">{help}</p>
      </div>
      <Switch checked={value} onCheckedChange={onChange} />
    </div>
  );
}

// OrphanCleanup scans for on-disk files that have no torrent in the listing
// (pre-existing leftovers, client "rem", delete-without-files) and removes them
// on demand, after previewing how many and how much they weigh.
function OrphanCleanup() {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);

  const run = async () => {
    setBusy(true);
    try {
      const { items, totalBytes } = await api.listOrphans();
      if (!items.length) {
        toast.info(t("cleanup.none"));
        return;
      }
      if (!confirm(t("cleanup.confirm", { count: items.length, size: fmtSize(totalBytes) }))) return;
      const { removed, freedBytes } = await api.cleanOrphans();
      toast.success(t("cleanup.done", { count: removed, size: fmtSize(freedBytes) }));
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <Label className="text-sm">{t("cleanup.orphans")}</Label>
        <p className="text-xs text-muted-foreground">{t("cleanup.orphansHelp")}</p>
      </div>
      <Button variant="outline" size="sm" className="shrink-0" onClick={run} disabled={busy}>
        {busy ? <Loader2 className="animate-spin" data-icon="inline-start" /> : <Trash2 data-icon="inline-start" />}
        {t("cleanup.scan")}
      </Button>
    </div>
  );
}
