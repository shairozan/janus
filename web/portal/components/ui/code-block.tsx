"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

interface CodeBlockProps {
  value: string;
  /** Short caption shown above the block (e.g. "PEM", "License JWT"). */
  label?: string;
  className?: string;
}

// A monospace block for keys / JWTs / JTIs with one-click copy. Uses the
// --font-mono token (font-mono). The copy affordance confirms inline so the
// staff user never wonders whether it worked.
export function CodeBlock({ value, label, className }: CodeBlockProps) {
  const [copied, setCopied] = React.useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard blocked (insecure context / permissions) — leave state as-is.
    }
  }

  return (
    <div className={cn("rounded-md border border-line bg-paper/70", className)}>
      <div className="flex items-center justify-between border-b border-line/70 px-3 py-1.5">
        <span className="font-sans text-[11px] uppercase tracking-[0.14em] text-ink-muted">
          {label ?? "Value"}
        </span>
        <button
          type="button"
          onClick={copy}
          className="font-sans text-xs font-medium text-ink-muted transition-colors hover:text-gold focus-visible:text-gold focus-visible:outline-none"
        >
          {copied ? "Copied ✓" : "Copy"}
        </button>
      </div>
      <pre className="max-h-64 overflow-auto px-3 py-2.5 font-mono text-xs leading-relaxed text-ink">
        <code className="break-all whitespace-pre-wrap">{value}</code>
      </pre>
    </div>
  );
}
