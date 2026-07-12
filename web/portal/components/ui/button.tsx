import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

// A small, reusable button foundation (shadcn/ui style: a cva recipe + cn()).
// The B3 screens reuse these variants so every action looks consistent.
const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        // primary: navy fill, warms to a richer navy on hover
        primary: "bg-brand text-paper hover:bg-ink",
        // secondary: paper with a gold hairline that fills on hover
        outline: "border border-gold/60 text-ink hover:bg-gold/10",
        // quiet: text-only
        ghost: "text-ink hover:bg-ink/5",
      },
      size: {
        sm: "h-9 px-3.5",
        md: "h-11 px-5",
        lg: "h-12 px-7 text-base",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
  )
);
Button.displayName = "Button";

export { buttonVariants };
