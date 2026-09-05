"use client";
import { useState } from "react";

export function CopyButton({ text, label }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable — no-op */
    }
  }
  return (
    <button
      onClick={copy}
      className="group flex max-w-full items-center gap-3 rounded-xl border border-line bg-chip px-4 py-3 text-left font-mono text-sm text-ink2 transition-colors hover:border-accent"
      aria-label="Copy install command"
    >
      <span className="break-all">{label ?? text}</span>
      <span className="shrink-0 text-xs text-muted group-hover:text-accent">{copied ? "Copied ✓" : "Copy"}</span>
    </button>
  );
}
