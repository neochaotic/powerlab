import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Merge Tailwind classes with clsx. Used by all shadcn-svelte components.
 */
export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}
