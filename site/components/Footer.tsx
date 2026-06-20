import { Flame } from "@/components/Flame";

export function Footer() {
  return (
    <footer className="border-t border-line">
      <div className="mx-auto flex max-w-content flex-col items-center gap-4 px-6 py-12 text-center">
        <div className="flex items-center gap-2">
          <Flame className="h-5 w-5 text-accent" />
          <span className="font-bold tracking-tight">Burnt</span>
        </div>
        <div className="flex gap-6 text-sm text-muted">
          <a href="https://github.com/mafex11/Burnt" className="hover:text-ink">GitHub</a>
          <a href="https://github.com/mafex11/Burnt/releases/latest" className="hover:text-ink">Releases</a>
          <a href="https://github.com/mafex11/Burnt/blob/main/LICENSE" className="hover:text-ink">License (MIT)</a>
        </div>
        <p className="mt-2 font-mono text-sm text-accent">How much have you burnt today?</p>
      </div>
    </footer>
  );
}
