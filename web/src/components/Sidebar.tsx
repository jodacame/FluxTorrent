import { Library, ScrollText, Settings, Waves, Recycle, DownloadCloud, CheckCircle2 } from "lucide-react";
import { useI18n } from "@/i18n";
import { cn } from "@/lib/utils";
import { Separator } from "@/components/ui/separator";
import type { View } from "@/App";

export type Filter = "all" | "streaming" | "sharing" | "downloading" | "completed";

interface Counts {
  all: number;
  streaming: number;
  sharing: number;
  downloading: number;
  completed: number;
}

export function Sidebar({
  view,
  onView,
  filter,
  onFilter,
  counts,
}: {
  view: View;
  onView: (v: View) => void;
  filter: Filter;
  onFilter: (f: Filter) => void;
  counts: Counts;
}) {
  const { t } = useI18n();

  const filters: { key: Filter; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
    { key: "all", label: t("filter.all"), icon: Library },
    { key: "streaming", label: t("filter.streaming"), icon: Waves },
    { key: "sharing", label: t("filter.sharing"), icon: Recycle },
    { key: "downloading", label: t("filter.downloading"), icon: DownloadCloud },
    { key: "completed", label: t("filter.completed"), icon: CheckCircle2 },
  ];

  return (
    <div className="flex h-full flex-col bg-sidebar text-sidebar-foreground">
      <nav className="flex flex-col gap-0.5 p-2 pt-3">
        <NavItem active={view === "library"} onClick={() => onView("library")} icon={Library} label={t("nav.library")} />
        <NavItem active={view === "rules"} onClick={() => onView("rules")} icon={ScrollText} label={t("nav.rules")} />
        <NavItem active={view === "settings"} onClick={() => onView("settings")} icon={Settings} label={t("nav.settings")} />
      </nav>

      {view === "library" && (
        <>
          <Separator className="my-1" />
          <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            {t("filter.all")}
          </div>
          <nav className="flex flex-col gap-0.5 p-2 pt-0">
            {filters.map((f) => (
              <button
                key={f.key}
                onClick={() => onFilter(f.key)}
                className={cn(
                  "flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors",
                  filter === f.key
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground"
                )}
              >
                <f.icon className="size-4" />
                <span className="flex-1 text-left">{f.label}</span>
                <span className="text-xs tabular text-muted-foreground">{counts[f.key]}</span>
              </button>
            ))}
          </nav>
        </>
      )}

    </div>
  );
}

function NavItem({
  active,
  onClick,
  icon: Icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium transition-colors",
        active ? "bg-primary/15 text-foreground" : "text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground"
      )}
    >
      <Icon className={cn("size-4", active && "text-primary")} />
      {label}
    </button>
  );
}
