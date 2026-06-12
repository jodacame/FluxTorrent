import { HelpCircle, HardDrive, Recycle, Lock, ScrollText, Gauge, Info } from "lucide-react";
import { useI18n } from "@/i18n";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogClose,
} from "@/components/ui/dialog";

// Reusable, multi-language help modal explaining each settings group in plain
// language (SPEC §11b: approachable for non-technical users).
export function HelpDialog() {
  const { t } = useI18n();

  const sections = [
    { icon: HardDrive, h: t("help.storageH"), p: t("help.storageP"), tone: "text-sky-400" },
    { icon: Recycle, h: t("help.seedingH"), p: t("help.seedingP"), tone: "text-emerald-400" },
    { icon: Lock, h: t("help.privateH"), p: t("help.privateP"), tone: "text-amber-400" },
    { icon: ScrollText, h: t("help.rulesH"), p: t("help.rulesP"), tone: "text-violet-400" },
    { icon: Gauge, h: t("help.speedH"), p: t("help.speedP"), tone: "text-rose-400" },
  ];

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" className="size-8" aria-label={t("help.open")}>
          <HelpCircle className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-xl gap-0 p-0">
        <DialogHeader className="border-b p-5">
          <DialogTitle className="flex items-center gap-2">
            <span className="text-primary">◖</span> {t("help.title")}
          </DialogTitle>
          <DialogDescription className="text-[13px] leading-relaxed">{t("help.intro")}</DialogDescription>
        </DialogHeader>

        <ScrollArea className="max-h-[60vh]">
          <div className="flex flex-col gap-5 p-5">
            {sections.map((s, i) => (
              <section key={i} className="flex gap-3">
                <div className={`mt-0.5 shrink-0 ${s.tone}`}>
                  <s.icon className="size-5" />
                </div>
                <div>
                  <h4 className="text-sm font-semibold">{s.h}</h4>
                  <p className="whitespace-pre-line text-[13px] leading-relaxed text-muted-foreground">{s.p}</p>
                </div>
              </section>
            ))}

            {/* prominent callout: modes can change per torrent */}
            <div className="flex gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
              <Info className="mt-0.5 size-5 shrink-0 text-amber-400" />
              <div>
                <h4 className="text-sm font-semibold text-amber-300">{t("help.storageNoteH")}</h4>
                <p className="text-[13px] leading-relaxed text-amber-100/80">{t("help.storageNoteP")}</p>
              </div>
            </div>
          </div>
        </ScrollArea>

        <DialogFooter className="border-t p-4">
          <DialogClose asChild>
            <Button>{t("help.close")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
