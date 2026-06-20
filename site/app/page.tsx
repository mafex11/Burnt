import { Nav } from "@/components/Nav";
import { Hero } from "@/components/Hero";
import { LogoStrip } from "@/components/LogoStrip";
import { Features } from "@/components/Features";
import { Demo } from "@/components/Demo";
import { Install } from "@/components/Install";
import { Footer } from "@/components/Footer";

export default function Home() {
  return (
    <>
      <Nav />
      <main id="top">
        <Hero />
        <LogoStrip />
        <Features />
        <Demo />
        <Install />
      </main>
      <Footer />
    </>
  );
}
