import { useCallback, useEffect, useRef, useState } from "react";

export function useLocalStorage<T>(key: string, initial: T): [T, (v: T | ((prev: T) => T)) => void] {
  const [state, setState] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      if (raw) return JSON.parse(raw) as T;
    } catch { /* ignore */ }
    return initial;
  });

  const ref = useRef(state);
  ref.current = state;

  const set = useCallback((v: T | ((prev: T) => T)) => {
    setState((prev) => (typeof v === "function" ? (v as (p: T) => T)(prev) : v));
  }, []);

  // Persist outside the state updater — setState updaters must be pure
  // (StrictMode double-invokes them in dev, which would double-write).
  useEffect(() => {
    try {
      localStorage.setItem(key, JSON.stringify(state));
    } catch { /* ignore quota / private mode */ }
  }, [key, state]);

  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === key && e.newValue) {
        try { setState(JSON.parse(e.newValue)); } catch { /* ignore */ }
      }
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [key]);

  return [state, set];
}
