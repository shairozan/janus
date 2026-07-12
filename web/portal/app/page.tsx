import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";

// app/page.tsx is the "/" route — a refined "inscription plate" landing. The
// real screens (onboarding, /me, admin) land in B3. It's a Server Component by
// default (no "use client") — rendered on the server, ships no JS.
export default function Home() {
  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col items-center justify-center px-6 py-16">
      <section className="relative w-full max-w-md rounded-2xl border border-line bg-surface px-10 py-14 text-center shadow-card">
        {/* arch + keystone motif, echoing the logo's doorway */}
        <svg
          aria-hidden
          viewBox="0 0 48 46"
          className="mx-auto mb-6 h-11 w-11 text-gold"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
        >
          <path d="M6 46 V22 a18 18 0 0 1 36 0 V46" />
          <path d="M24 2.5 l3.5 3.5 -3.5 3.5 -3.5 -3.5 z" fill="currentColor" stroke="none" />
        </svg>

        <h1 className="text-balance font-display text-4xl leading-tight text-ink sm:text-5xl">
          Janus <span className="font-normal italic text-ink-muted">Management Portal</span>
        </h1>

        {/* short gold rule */}
        <div aria-hidden className="mx-auto mt-6 h-px w-12 bg-gold/70" />

        <p className="mx-auto mt-6 max-w-xs text-pretty text-sm leading-relaxed text-ink-muted">
          Manage your organization&apos;s licenses, signing keys, and members — every threshold, one
          doorway.
        </p>

        {/* Sign-in is wired to Auth.js in B2; /login lands then. */}
        <Link href="/login" className={`${buttonVariants({ size: "lg" })} mt-8 w-full`}>
          Sign in
        </Link>
      </section>
    </main>
  );
}
