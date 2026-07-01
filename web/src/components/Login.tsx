import { useState } from "react";
import { Loader2, Lock } from "lucide-react";
import { api } from "../api";
import { useI18n } from "../i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// Login is the single-password gate shown when FT_AUTH_PASSWORD is set on the
// server and the current browser has no valid session.
export function Login({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useI18n();
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy || !password) return;
    setBusy(true);
    setError("");
    try {
      await api.login(password);
      onSuccess();
    } catch (err) {
      setError((err as Error).message || t("login.failed"));
      setPassword("");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex h-screen items-center justify-center bg-background p-4 text-foreground">
      <form
        onSubmit={submit}
        className="w-full max-w-sm space-y-5 rounded-xl border bg-card/60 p-6 shadow-sm"
      >
        <div className="flex select-none flex-col items-center gap-2 text-center">
          <div className="flex size-11 items-center justify-center rounded-full bg-primary/10 text-primary">
            <Lock className="size-5" />
          </div>
          <div className="text-lg font-medium">
            Flux<b className="text-primary">Torrent</b>
          </div>
          <p className="text-sm text-muted-foreground">{t("login.subtitle")}</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="ft-password">{t("login.password")}</Label>
          <Input
            id="ft-password"
            type="password"
            autoFocus
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("login.placeholder")}
            aria-invalid={!!error}
          />
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <Button type="submit" className="w-full" disabled={busy || !password}>
          {busy ? <Loader2 className="animate-spin" data-icon="inline-start" /> : null}
          {t("login.submit")}
        </Button>
      </form>
    </div>
  );
}
