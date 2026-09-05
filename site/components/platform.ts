"use client";
import { useEffect, useSyncExternalStore } from "react";

export type Platform = "macos" | "windows";

export const PLATFORM_LABEL: Record<Platform, string> = {
  macos: "macOS",
  windows: "Windows",
};

export const PLATFORMS: Platform[] = ["macos", "windows"];

/**
 * A tiny module-level store so every PlatformToggle on the page (Hero, Install)
 * stays in sync without threading a provider through the server components.
 * SSR always renders macOS; detection happens after mount to keep hydration clean.
 */
let current: Platform = "macos";
let detected = false;
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

const getSnapshot = () => current;
const getServerSnapshot = (): Platform => "macos";

export function setPlatform(next: Platform) {
  detected = true;
  if (next === current) return;
  current = next;
  listeners.forEach((l) => l());
}

/** Reads the OS from the URL hash first (shareable links), then the browser. */
export function detectPlatform(): Platform {
  if (typeof window === "undefined") return "macos";
  const hash = window.location.hash.replace(/^#/, "").toLowerCase();
  if (hash === "windows" || hash === "macos") return hash;
  const nav = window.navigator as Navigator & { userAgentData?: { platform?: string } };
  const hint = nav.userAgentData?.platform ?? "";
  if (hint) return /win/i.test(hint) ? "windows" : "macos";
  return /windows|win32|win64/i.test(nav.userAgent ?? "") ? "windows" : "macos";
}

export function usePlatform(): [Platform, (next: Platform) => void] {
  const platform = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  useEffect(() => {
    if (detected) return;
    setPlatform(detectPlatform());
  }, []);
  return [platform, setPlatform];
}

/** Test-only: drop the shared state between cases. */
export function resetPlatformStore() {
  current = "macos";
  detected = false;
  listeners.clear();
}
