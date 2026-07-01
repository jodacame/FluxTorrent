import React, { useCallback, useEffect, useState } from "react";
import ReactDOM from "react-dom/client";
import { Loader2 } from "lucide-react";
import App from "./App";
import { Login } from "./components/Login";
import { api, setUnauthorizedHandler } from "./api";
import { I18nProvider } from "./i18n";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import "./index.css";

// AuthGate resolves the server's auth state on load and renders either the
// login screen or the app. It also catches mid-session 401s (expired cookie)
// and drops back to login. When no password is configured server-side, it falls
// straight through to the app.
function AuthGate() {
  const [phase, setPhase] = useState<"loading" | "login" | "ready">("loading");
  const [enabled, setEnabled] = useState(false);

  const resolve = useCallback(() => {
    api
      .authStatus()
      .then((s) => {
        setEnabled(s.required);
        setPhase(s.required && !s.authenticated ? "login" : "ready");
      })
      // If the status check itself fails, don't hard-lock the UI — let normal
      // API calls surface a 401 and flip us to login if needed.
      .catch(() => setPhase("ready"));
  }, []);

  useEffect(() => {
    resolve();
    setUnauthorizedHandler(() => setPhase("login"));
    return () => setUnauthorizedHandler(null);
  }, [resolve]);

  if (phase === "loading") {
    return (
      <div className="flex h-screen items-center justify-center bg-background text-muted-foreground">
        <Loader2 className="size-6 animate-spin" />
      </div>
    );
  }
  if (phase === "login") {
    return <Login onSuccess={() => setPhase("ready")} />;
  }
  return (
    <App
      authEnabled={enabled}
      onLogout={async () => {
        await api.logout().catch(() => {});
        setPhase("login");
      }}
    />
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>
      <TooltipProvider delayDuration={300}>
        <AuthGate />
        <Toaster position="bottom-right" theme="dark" richColors />
      </TooltipProvider>
    </I18nProvider>
  </React.StrictMode>
);
