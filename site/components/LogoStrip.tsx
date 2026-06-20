export function LogoStrip() {
  return (
    <div className="border-y border-line bg-panel/50">
      <p className="mx-auto max-w-content px-6 py-4 text-center text-sm text-muted">
        Works with <span className="text-ink">Claude Code</span> ·{" "}
        <span className="text-ink">Codex</span> · powered by{" "}
        <a href="https://github.com/ryoppippi/ccusage" className="text-ink underline decoration-line underline-offset-2 hover:text-accent">ccusage</a>
      </p>
    </div>
  );
}
