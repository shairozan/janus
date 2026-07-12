"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

// Nav item that marks itself active based on the current path (so the shell can
// stay a Server Component while the highlight is client-driven).
export function NavLink({ href, children }: { href: string; children: React.ReactNode }) {
  const pathname = usePathname();
  const active = pathname === href || pathname.startsWith(`${href}/`);

  return (
    <Link
      href={href}
      data-active={active}
      className={cn(
        "border-b-2 border-transparent px-2 py-3 text-sm font-medium text-ink-muted transition-colors hover:text-ink",
        "data-[active=true]:border-gold data-[active=true]:text-ink"
      )}
    >
      {children}
    </Link>
  );
}
