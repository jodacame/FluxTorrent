import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { I18nProvider } from "./i18n";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>
      <TooltipProvider delayDuration={300}>
        <App />
        <Toaster position="bottom-right" theme="dark" richColors />
      </TooltipProvider>
    </I18nProvider>
  </React.StrictMode>
);
