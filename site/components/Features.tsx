const FEATURES: { label: string; title: string; desc: string }[] = [
  { label: "Spend", title: "Today / week / month / all-time", desc: "Cost in USD and token volume, every timeframe at a glance." },
  { label: "Momentum", title: "Trend & pace", desc: "How this week compares to last, and where today is heading." },
  { label: "History", title: "14-day sparkline + 12-week heatmap", desc: "See your burn over time; hover any day for its exact cost." },
  { label: "Split", title: "By tool", desc: "Claude Code vs Codex, color-coded so you know who's spending." },
  { label: "Models", title: "By model", desc: "Where the expensive tokens go — opus, sonnet, gpt-5, and more." },
  { label: "Projects", title: "By project", desc: "Which repo or directory is quietly eating your budget." },
  { label: "Savings", title: "Cache savings", desc: "An estimate of how much prompt caching saved you." },
  { label: "Share", title: "Burnt Wrapped", desc: "A shareable card of your month or all-time spend — copy or save as PNG." },
];

export function Features() {
  return (
    <section id="features" className="mx-auto max-w-content px-6 py-24">
      <h2 className="max-w-2xl text-3xl font-extrabold tracking-tight md:text-4xl">
        Everything your spend is hiding, one click away.
      </h2>
      <p className="mt-3 max-w-xl text-muted">
        Click the menu-bar flame for a full breakdown — accurate to the cent, because
        ccusage is bundled right in.
      </p>
      <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {FEATURES.map((f) => (
          <div key={f.title} className="rounded-2xl border border-line bg-card p-6">
            <div className="text-xs font-semibold uppercase tracking-[0.1em] text-accent">{f.label}</div>
            <h3 className="mt-2 text-lg font-bold tracking-tight">{f.title}</h3>
            <p className="mt-2 text-sm text-muted">{f.desc}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
