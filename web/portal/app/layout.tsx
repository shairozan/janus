import type { Metadata } from "next";
import { Fraunces, Hanken_Grotesk, IBM_Plex_Mono } from "next/font/google";
import Image from "next/image";
import Link from "next/link";
import "./globals.css";

// Distinctive type pairing: Fraunces (a soft, high-character old-style serif)
// for display/headings, Hanken Grotesk (a warm humanist sans) for UI/body text.
// next/font self-hosts these at build time and exposes them as CSS variables
// consumed by tailwind.config.ts (font-display / font-sans).
const display = Fraunces({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  style: ["normal", "italic"],
  variable: "--font-display",
  display: "swap",
});

const sans = Hanken_Grotesk({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

// Mono — IBM Plex Mono — for signing keys, JWTs, and JTIs (the design standards
// anticipate a third face for keys/JTIs; added as a token, not inlined).
const mono = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  variable: "--font-mono",
  display: "swap",
});

// In the App Router, this root layout wraps EVERY page: it renders the <html>
// and <body> shell once, plus the shared header. Routes render into {children}.
// app/icon.png (the Janus logo) is automatically used as the browser tab icon.
export const metadata: Metadata = {
  title: "Janus Management Portal",
  description: "Manage your Janus licenses, signing keys, and organization.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${display.variable} ${sans.variable} ${mono.variable}`}>
      <body className="flex min-h-screen flex-col font-sans antialiased">
        {/* gold-leaf edge across the very top */}
        <div aria-hidden className="h-[3px] bg-gradient-to-r from-gold/0 via-gold to-gold/0" />

        <header className="sticky top-0 z-40 border-b border-line bg-paper/80 backdrop-blur supports-[backdrop-filter]:bg-paper/65">
          <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-3.5">
            <Link href="/" className="group flex items-center gap-3">
              {/* next/image optimizes + resizes the source PNG on demand */}
              <Image
                src="/logo.png"
                alt="Janus"
                width={34}
                height={34}
                priority
                className="rounded-[6px] ring-1 ring-line"
              />
              <span className="font-display text-xl font-medium tracking-tight text-brand">
                Janus
              </span>
            </Link>
            <span className="hidden font-sans text-[11px] uppercase tracking-[0.18em] text-ink-muted sm:block">
              Management Portal
            </span>
          </div>
        </header>

        <div className="flex flex-1 flex-col">{children}</div>
      </body>
    </html>
  );
}
