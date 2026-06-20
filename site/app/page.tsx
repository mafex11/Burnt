import { Nav } from "@/components/Nav";
import { Hero } from "@/components/Hero";
import { LogoStrip } from "@/components/LogoStrip";
import { Features } from "@/components/Features";

export default function Home() {
  return (
    <>
      <Nav />
      <main id="top">
        <Hero />
        <LogoStrip />
        <Features />
      </main>
    </>
  );
}
