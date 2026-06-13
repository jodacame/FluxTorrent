import { useEffect, useState } from "react";
import { Plus, Trash2, GripVertical, Save } from "lucide-react";
import { toast } from "sonner";
import { api, type Rule, type RuleAction, type RuleField, type RuleOp } from "@/api";
import { useI18n } from "@/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const FIELDS: RuleField[] = ["name", "tracker", "indexer"];
const OPS: RuleOp[] = ["contains", "equals", "regex"];
const ACTIONS: RuleAction[] = ["keepSeed", "forceDisk", "forceRam", "prefer", "reject"];

function blankRule(): Rule {
  return { match: { field: "tracker", op: "contains", value: "" }, action: "keepSeed", seed: { maxRatio: 2, maxMinutes: 0 }, note: "" };
}

export function RulesEditor() {
  const { t } = useI18n();
  const [rules, setRules] = useState<Rule[]>([]);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    api.getRules().then(setRules).catch(() => {});
  }, []);

  const update = (i: number, r: Rule) => {
    setRules((rs) => rs.map((x, j) => (j === i ? r : x)));
    setDirty(true);
  };
  const remove = (i: number) => {
    setRules((rs) => rs.filter((_, j) => j !== i));
    setDirty(true);
  };
  const addRule = () => {
    setRules((rs) => [...rs, blankRule()]);
    setDirty(true);
  };
  const save = async () => {
    try {
      const saved = await api.putRules(rules);
      setRules(saved);
      setDirty(false);
      toast.success(t("rules.saved"));
    } catch (e) {
      toast.error((e as Error).message);
    }
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex shrink-0 items-center gap-3 border-b bg-card/60 px-4 py-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">{t("rules.title")}</h2>
          <p className="truncate text-xs text-muted-foreground">{t("rules.subtitle")}</p>
        </div>
        <div className="ml-auto flex shrink-0 gap-2">
          <Button variant="outline" size="sm" onClick={addRule}>
            <Plus data-icon="inline-start" /> <span className="hidden sm:inline">{t("rules.add")}</span>
          </Button>
          <Button size="sm" onClick={save} disabled={!dirty}>
            <Save data-icon="inline-start" /> <span className="hidden sm:inline">{t("rules.save")}</span>
          </Button>
        </div>
      </div>

      <ScrollArea className="min-h-0 flex-1">
        <div className="flex flex-col gap-3 p-4">
          {rules.length === 0 && (
            <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
              {t("rules.empty")}
            </div>
          )}
          {rules.map((r, i) => (
            <RuleRow key={i} rule={r} index={i} onChange={(x) => update(i, x)} onRemove={() => remove(i)} />
          ))}
        </div>
      </ScrollArea>
    </div>
  );
}

function RuleRow({
  rule,
  index,
  onChange,
  onRemove,
}: {
  rule: Rule;
  index: number;
  onChange: (r: Rule) => void;
  onRemove: () => void;
}) {
  const { t } = useI18n();
  const isKeepSeed = rule.action === "keepSeed";

  return (
    <div className="rounded-lg border bg-card p-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="hidden size-6 items-center justify-center text-muted-foreground sm:flex">
          <GripVertical className="size-4" />
        </div>
        <span className="w-5 text-xs tabular text-muted-foreground">{index + 1}</span>

        {/* match field */}
        <Select value={rule.match.field} onValueChange={(v) => onChange({ ...rule, match: { ...rule.match, field: v as RuleField } })}>
          <SelectTrigger className="h-8 w-[110px]"><SelectValue /></SelectTrigger>
          <SelectContent>
            {FIELDS.map((f) => <SelectItem key={f} value={f}>{t(`field.${f}`)}</SelectItem>)}
          </SelectContent>
        </Select>

        <Select value={rule.match.op} onValueChange={(v) => onChange({ ...rule, match: { ...rule.match, op: v as RuleOp } })}>
          <SelectTrigger className="h-8 w-[110px]"><SelectValue /></SelectTrigger>
          <SelectContent>
            {OPS.map((o) => <SelectItem key={o} value={o}>{t(`op.${o}`)}</SelectItem>)}
          </SelectContent>
        </Select>

        <Input
          value={rule.match.value}
          onChange={(e) => onChange({ ...rule, match: { ...rule.match, value: e.target.value } })}
          placeholder={t("rules.value")}
          className="h-8 min-w-[140px] flex-1"
        />

        <span className="hidden text-muted-foreground sm:inline">→</span>

        <Select value={rule.action} onValueChange={(v) => onChange({ ...rule, action: v as RuleAction })}>
          <SelectTrigger className="h-8 w-[150px]"><SelectValue /></SelectTrigger>
          <SelectContent>
            {ACTIONS.map((a) => <SelectItem key={a} value={a}>{t(`action.${a}`)}</SelectItem>)}
          </SelectContent>
        </Select>

        <Button variant="ghost" size="icon" className="size-8 shrink-0 hover:text-destructive" onClick={onRemove}>
          <Trash2 className="size-4" />
        </Button>
      </div>

      {isKeepSeed && (
        <>
          <Separator className="my-2" />
          <div className="flex flex-wrap items-center gap-4 pl-0 text-[13px] sm:pl-10">
            <label className="flex items-center gap-2">
              <span className="text-muted-foreground">{t("rules.ratio")}</span>
              <Input
                type="number"
                step="0.1"
                value={rule.seed?.maxRatio ?? 0}
                onChange={(e) => onChange({ ...rule, seed: { maxRatio: Number(e.target.value), maxMinutes: rule.seed?.maxMinutes ?? 0 } })}
                className="h-7 w-20"
              />
            </label>
            <label className="flex items-center gap-2">
              <span className="text-muted-foreground">{t("rules.minutes")}</span>
              <Input
                type="number"
                value={rule.seed?.maxMinutes ?? 0}
                onChange={(e) => onChange({ ...rule, seed: { maxRatio: rule.seed?.maxRatio ?? 0, maxMinutes: Number(e.target.value) } })}
                className="h-7 w-24"
              />
            </label>
            <span className="text-xs text-muted-foreground">0 = ∞</span>
          </div>
        </>
      )}

      <div className="mt-2 flex flex-wrap items-center gap-2 pl-0 sm:pl-10">
        <label className="flex shrink-0 items-center gap-2 text-[13px]">
          <span className="text-muted-foreground">{t("rules.maxConns")}</span>
          <Input
            type="number"
            value={rule.maxConns ?? 0}
            onChange={(e) => onChange({ ...rule, maxConns: Number(e.target.value) })}
            className="h-7 w-20"
          />
        </label>
        <Input
          value={rule.note}
          onChange={(e) => onChange({ ...rule, note: e.target.value })}
          placeholder={t("rules.note")}
          className="h-7 flex-1 text-xs"
        />
      </div>
    </div>
  );
}
