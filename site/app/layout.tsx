import type { Metadata } from "next";
import { Manrope, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const manrope = Manrope({ subsets: ["latin"], variable: "--font-sans", weight: ["400","500","600","700","800"] });
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono", weight: ["400","500","700"] });

export const metadata: Metadata = {
  title: "Burnt — see what you've burnt on Claude Code & Codex",
  description: "A menu bar and system tray tracker for Claude Code and Codex token usage and cost, for macOS and Windows. Real-dollar spend, today / week / month, at a glance.",
  icons: { icon: "/appicon.png" },
  openGraph: {
    title: "Burnt — menu bar & system tray cost tracker for Claude Code & Codex",
    description: "Real-dollar cost and token usage, right in your menu bar or system tray.",
    images: ["/appicon.png"],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${manrope.variable} ${mono.variable}`}>
      <body className="font-sans">{children}</body>
    </html>
  );
}
