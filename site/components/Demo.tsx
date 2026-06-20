"use client";
import { useEffect, useRef } from "react";

export function Demo() {
  const videoRef = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const v = videoRef.current;
    if (!v) return;
    if (reduce) {
      v.removeAttribute("autoplay");
      v.pause();
    } else {
      v.play().catch(() => {});
    }
  }, []);

  return (
    <section className="bg-ink text-paper">
      <div className="mx-auto max-w-content px-6 py-24">
        <h2 className="text-3xl font-extrabold tracking-tight md:text-4xl">Watch it land in your menu bar.</h2>
        <p className="mt-3 max-w-xl text-paper/60">From one brew command to a live flame with today&apos;s spend.</p>
        <div className="mt-10 overflow-hidden rounded-2xl border border-white/10 bg-black/40">
          <div className="flex items-center gap-2 border-b border-white/10 px-4 py-3">
            <span className="h-3 w-3 rounded-full bg-[#ff5f57]" />
            <span className="h-3 w-3 rounded-full bg-[#febc2e]" />
            <span className="h-3 w-3 rounded-full bg-[#28c840]" />
          </div>
          <video
            ref={videoRef}
            className="w-full"
            muted loop playsInline
            poster="/hero.png"
            aria-label="Burnt launch demo"
          >
            <source src="/burnt-launch.mp4" type="video/mp4" />
          </video>
        </div>
      </div>
    </section>
  );
}
