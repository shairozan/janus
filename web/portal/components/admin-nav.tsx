"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import * as React from "react";
import { cn } from "@/lib/utils";

// The Janus-staff "Admin" menu. Distinct from the per-org customer-admin nav:
// this is the licensing authority's inner sanctum (signing keys + license
// issuance), shown only when Profile.is_staff. A small gold keystone marks it.
const ITEMS = [
  { href: "/admin/keys", label: "Signing Keys" },
  { href: "/admin/agreements", label: "Agreements" },
  { href: "/admin/licenses", label: "Issue License" },
];

export function AdminNav() {
  const pathname = usePathname();
  const [open, setOpen] = React.useState(false);
  const ref = React.useRef<HTMLDivElement>(null);
  const active = pathname.startsWith("/admin");

  // Close on outside click or Escape.
  React.useEffect(() => {
    if (!open) return;

    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);

    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        data-active={active}
        className={cn(
          "flex items-center gap-1.5 border-b-2 border-transparent px-2 py-3 text-sm font-medium text-ink-muted transition-colors hover:text-ink",
          "data-[active=true]:border-gold data-[active=true]:text-ink"
        )}
      >
        {/* keystone motif */}
        <span aria-hidden className="text-gold">
          ⌂
        </span>
        Admin
        <span aria-hidden className={cn("text-[10px] transition-transform", open && "rotate-180")}>
          ▾
        </span>
      </button>

      {open && (
        <div
          role="menu"
          className="absolute left-0 top-full z-50 mt-1 min-w-44 overflow-hidden rounded-xl border border-line bg-surface shadow-card"
        >
          <p className="border-b border-line px-3 py-2 font-display text-[11px] uppercase tracking-[0.16em] text-ink-muted">
            Janus Staff
          </p>
          {ITEMS.map((item) => {
            const isActive = pathname === item.href || pathname.startsWith(`${item.href}/`);

            return (
              <Link
                key={item.href}
                href={item.href}
                role="menuitem"
                onClick={() => setOpen(false)}
                data-active={isActive}
                className={cn(
                  "block px-3 py-2 text-sm text-ink transition-colors hover:bg-gold/10",
                  "data-[active=true]:bg-gold/10 data-[active=true]:text-ink"
                )}
              >
                {item.label}
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
