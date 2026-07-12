import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

// Callout for important notices — e.g. the key-rotation caveat (warning tone).
const alertVariants = cva("rounded-lg border px-4 py-3 text-sm", {
  variants: {
    tone: {
      info: "border-brand/20 bg-brand/5 text-ink",
      warning: "border-gold/40 bg-gold/10 text-[#5c4410]",
      danger: "border-red-200 bg-red-50 text-red-800",
    },
  },
  defaultVariants: { tone: "info" },
});

export interface AlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof alertVariants> {}

export function Alert({ className, tone, role = "note", ...props }: AlertProps) {
  return <div role={role} className={cn(alertVariants({ tone }), className)} {...props} />;
}

export function AlertTitle({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("font-medium", className)} {...props} />;
}
