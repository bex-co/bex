import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Merge class names with Tailwind CSS conflict resolution
 * Combines multiple class names and resolves Tailwind CSS conflicts
 * @param inputs - Class names to merge (strings, arrays, objects, etc.)
 * @returns Merged class names as a single string
 * @example
 * cn("px-2", "py-1") // => "px-2 py-1"
 * cn("px-2", "px-4") // => "px-4" (later class wins)
 * cn("base", { active: true, disabled: false }) // => "base active"
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
