import { Flame } from "@/components/Flame";

export function Nav() {
  return (
    <header className="sticky top-0 z-50 border-b border-line bg-paper/80 backdrop-blur">
      <nav className="mx-auto flex max-w-content items-center justify-between px-6 py-4">
        <a href="#top" className="flex items-center gap-2">
          <Flame className="h-6 w-6 text-accent" />
          <span className="text-lg font-bold tracking-tight">Burnt</span>
        </a>
        <div className="flex items-center gap-6 text-sm text-muted">
          <a href="#features" className="hover:text-ink">Features</a>
          <a href="#install" className="hover:text-ink">Install</a>
          <a href="https://github.com/mafex11/Burnt" className="hover:text-ink">GitHub</a>
          <span className="rounded-full border border-line px-3 py-1 text-xs text-ink2">macOS 14+</span>
        </div>
      </nav>
    </header>
  );
}
