import type { Config } from "tailwindcss";

// Tailwind scans these files for class names and only emits the CSS actually
// used. Colors + fonts map to the CSS variables defined in app/globals.css and
// the next/font variables set in app/layout.tsx, so utilities like `text-ink`,
// `bg-paper`, `border-line`, `text-gold`, and `font-display` are available
// everywhere and re-theme from one place.
const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        paper: "var(--paper)",
        surface: "var(--surface)",
        brand: "var(--brand)",
        line: "var(--line)",
        ink: {
          DEFAULT: "var(--ink)",
          muted: "var(--ink-muted)",
        },
        gold: {
          DEFAULT: "var(--gold)",
          soft: "var(--gold-soft)",
        },
      },
      fontFamily: {
        display: ["var(--font-display)", "Georgia", "serif"],
        sans: ["var(--font-sans)", "system-ui", "sans-serif"],
        mono: ["var(--font-mono)", "ui-monospace", "SFMono-Regular", "monospace"],
      },
      boxShadow: {
        // soft, warm-tinted elevation rather than harsh black shadows
        card: "0 1px 2px rgba(22, 49, 79, 0.04), 0 8px 24px -12px rgba(22, 49, 79, 0.12)",
      },
    },
  },
  plugins: [],
};

export default config;
