import type { Config } from "tailwindcss";
const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        paper: "var(--bg)", panel: "var(--panel)", card: "var(--card)",
        ink: "var(--ink)", ink2: "var(--ink-2)", muted: "var(--muted)",
        line: "var(--line)", chip: "var(--chip)",
        accent: "var(--accent)", accentBright: "var(--accent-bright)", pos: "var(--pos)",
      },
      fontFamily: {
        sans: ["var(--font-sans)", "system-ui", "sans-serif"],
        mono: ["var(--font-mono)", "monospace"],
      },
      maxWidth: { content: "1120px" },
    },
  },
  plugins: [],
};
export default config;
