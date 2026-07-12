import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * cn merges Tailwind class names, resolving conflicts (later wins). The standard
 * helper used by shadcn/ui components, e.g. cn("p-2", isActive && "bg-blue-500").
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
