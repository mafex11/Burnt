const RAMP = ["#ede6e6","#f5cdd5","#e89aa6","#db6678","#cc3a52","#c01933"];
// deterministic illustrative pattern (no RNG → stable static export)
const cells = Array.from({ length: 84 }, (_, i) =>
  RAMP[Math.floor(Math.pow((i * 37 % 84) / 84, 1.4) * RAMP.length) % RAMP.length]
);
const bars = [90, 55, 30];

export function PopoverMock() {
  return (
    <div className="w-[280px] rounded-2xl border border-line bg-card p-4 shadow-[0_24px_60px_rgba(0,0,0,0.10)]">
      <div className="text-3xl font-bold tracking-tight">
        $48.20 <span className="align-middle text-sm font-semibold text-pos">↑ today</span>
      </div>
      <div className="mt-1 mb-3 text-xs text-muted">week $214 · month $812</div>
      <div className="flex flex-col gap-2">
        {bars.map((w, i) => (
          <div key={i} className="h-2 overflow-hidden rounded bg-chip">
            <div className="h-full rounded" style={{ width: `${w}%`, background: "linear-gradient(90deg,#e2344b,#c01933)" }} />
          </div>
        ))}
      </div>
      <div className="mt-3 grid grid-cols-12 gap-[3px]">
        {cells.map((c, i) => (
          <div key={i} className="aspect-square rounded-[2px]" style={{ background: c }} />
        ))}
      </div>
    </div>
  );
}
