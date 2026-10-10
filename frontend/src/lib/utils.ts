import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merges conditional class names and resolves conflicting Tailwind utilities so
 * the last one wins. Without the merge step, a component default would beat a
 * caller override.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
