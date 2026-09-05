import { PopoverMock } from "@/components/PopoverMock";
import { HeroInstall } from "@/components/PlatformInstall";

export function Hero() {
  return (
    <section className="mx-auto grid max-w-content items-center gap-12 px-6 py-20 md:grid-cols-2 md:py-28">
      <div>
        <div className="mb-4 text-xs font-semibold uppercase tracking-[0.16em] text-accent">
          Menu bar &amp; system tray cost tracker
        </div>
        <h1 className="text-5xl font-extrabold leading-[1.05] tracking-tight md:text-6xl">
          Know what you&apos;ve <span className="text-accent">burnt</span>.
        </h1>
        <p className="mt-5 max-w-md text-lg text-muted">
          Real-dollar cost and token usage for Claude Code and Codex — today, this week,
          this month — right in your menu bar or system tray.
        </p>
        <div className="mt-7">
          <HeroInstall />
        </div>
      </div>
      <div className="flex justify-center md:justify-end">
        <PopoverMock />
      </div>
    </section>
  );
}
