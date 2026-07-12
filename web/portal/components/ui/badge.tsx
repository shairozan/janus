import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

// Status pill. Tones map to the portal's state vocabulary (approved/pending/etc.).
const badgeVariants = cva(
  "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
  {
    variants: {
      tone: {
        neutral: "bg-ink/5 text-ink-muted",
        success: "bg-emerald-50 text-emerald-700",
        warning: "bg-gold/15 text-[#7a5c12]",
        danger: "bg-red-50 text-red-700",
        info: "bg-brand/10 text-brand",
      },
    },
    defaultVariants: { tone: "neutral" },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, tone, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
