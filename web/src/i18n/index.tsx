// Lightweight i18n — no external dependency.
//
// Adding a language:
//   1. create src/i18n/locales/<code>.ts with the same keys as es.ts
//   2. import it below and add it to LOCALES with a display name
// The UI language switcher and persistence pick it up automatically.

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { es } from "./locales/es";
import { en } from "./locales/en";

export type Dict = Record<string, string>;

export interface Locale {
  code: string;
  name: string; // native display name
  dict: Dict;
}

// Registry of available languages (order = switcher order).
export const LOCALES: Locale[] = [
  { code: "es", name: "Español", dict: es },
  { code: "en", name: "English", dict: en },
];

const STORAGE_KEY = "ft_lang";

function detect(): string {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved && LOCALES.some((l) => l.code === saved)) return saved;
  const nav = navigator.language.slice(0, 2).toLowerCase();
  return LOCALES.some((l) => l.code === nav) ? nav : "es";
}

interface I18nCtx {
  lang: string;
  setLang: (code: string) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
  locales: Locale[];
}

const Ctx = createContext<I18nCtx | null>(null);

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = useState<string>(detect);

  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);

  const setLang = useCallback((code: string) => {
    localStorage.setItem(STORAGE_KEY, code);
    setLangState(code);
  }, []);

  const t = useCallback(
    (key: string, vars?: Record<string, string | number>) => {
      const loc = LOCALES.find((l) => l.code === lang) ?? LOCALES[0];
      let s = loc.dict[key] ?? en[key] ?? key;
      if (vars) {
        for (const [k, v] of Object.entries(vars)) {
          s = s.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
        }
      }
      return s;
    },
    [lang]
  );

  const value = useMemo<I18nCtx>(() => ({ lang, setLang, t, locales: LOCALES }), [lang, setLang, t]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useI18n(): I18nCtx {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useI18n must be used within I18nProvider");
  return ctx;
}
