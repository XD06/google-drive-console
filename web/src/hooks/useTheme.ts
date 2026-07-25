import { useCallback, useEffect } from "react";
import { useLocalStorage } from "./useLocalStorage";

export type ThemeMode = "auto" | "light" | "dark";

export function useTheme() {
  const [mode, setMode] = useLocalStorage<ThemeMode>("dbc.theme", "auto");

  const apply = useCallback(() => {
    const root = document.documentElement;
    const isDark = mode === "dark" || (mode === "auto" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    root.setAttribute("data-theme", isDark ? "dark" : "light");
  }, [mode]);

  useEffect(() => {
    apply();
  }, [apply]);

  useEffect(() => {
    if (mode !== "auto") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => apply();
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [mode, apply]);

  return { mode, setMode };
}
