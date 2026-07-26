import { useEffect } from "react";
import { useLocalStorage } from "./useLocalStorage";

export type WallpaperId = "aurora" | "sakura" | "minimal";

/**
 * Wallpaper behind the glass panels (Settings → Appearance).
 * Reflected as html[data-wallpaper]; CSS picks the light/dark asset
 * based on html[data-theme]. "minimal" keeps the animated gradient field.
 */
export function useWallpaper() {
  const [wallpaper, setWallpaper] = useLocalStorage<WallpaperId>("dbc.wallpaper", "aurora");

  useEffect(() => {
    document.documentElement.setAttribute("data-wallpaper", wallpaper);
  }, [wallpaper]);

  return { wallpaper, setWallpaper };
}
