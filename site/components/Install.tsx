import { CopyButton } from "@/components/CopyButton";

export function Install() {
  return (
    <section id="install" className="mx-auto max-w-content px-6 py-24 text-center">
      <h2 className="text-3xl font-extrabold tracking-tight md:text-4xl">Install in one command.</h2>
      <p className="mx-auto mt-3 max-w-md text-muted">
        No Node, no dependencies, works offline. Burnt installs, strips quarantine, and
        launches itself straight into your menu bar.
      </p>
      <div className="mt-8 flex justify-center">
        <CopyButton text="brew install mafex11/tap/burnt" />
      </div>
      <p className="mt-4 font-mono text-xs text-muted">
        update later: brew upgrade --cask mafex11/tap/burnt
      </p>
      <p className="mx-auto mt-8 max-w-md text-sm text-muted">
        Prefer a direct download? Grab <code className="text-ink">Burnt.zip</code> from the{" "}
        <a href="https://github.com/mafex11/Burnt/releases/latest" className="text-ink underline decoration-line underline-offset-2 hover:text-accent">latest release</a>.
        Burnt is ad-hoc signed, so the first time, right-click the app → <span className="text-ink">Open</span>.
      </p>
    </section>
  );
}
