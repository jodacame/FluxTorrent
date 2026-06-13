import { useEffect, useState } from "react";

// useMediaQuery tracks a CSS media query and re-renders on changes.
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window !== "undefined" ? window.matchMedia(query).matches : false
  );

  useEffect(() => {
    const mql = window.matchMedia(query);
    const onChange = () => setMatches(mql.matches);
    onChange();
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}

// useIsDesktop is true at the `lg` breakpoint (1024px) and up, where the full
// resizable three-pane layout is shown; below it the UI switches to drawers.
export function useIsDesktop(): boolean {
  return useMediaQuery("(min-width: 1024px)");
}
