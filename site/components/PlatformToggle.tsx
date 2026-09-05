"use client";
import { PLATFORMS, PLATFORM_LABEL, usePlatform } from "@/components/platform";

export function PlatformToggle({ className = "" }: { className?: string }) {
  const [platform, choose] = usePlatform();
  return (
    <div
      role="tablist"
      aria-label="Choose your platform"
      className={`inline-flex items-center gap-1 rounded-xl border border-line bg-chip p-1 text-sm ${className}`}
    >
      {PLATFORMS.map((p) => {
        const selected = p === platform;
        return (
          <button
            key={p}
            type="button"
            role="tab"
            aria-selected={selected}
            onClick={() => choose(p)}
            className={
              selected
                ? "rounded-lg bg-card px-3 py-1.5 font-semibold text-ink"
                : "rounded-lg px-3 py-1.5 text-muted transition-colors hover:text-ink"
            }
          >
            {PLATFORM_LABEL[p]}
          </button>
        );
      })}
    </div>
  );
}
