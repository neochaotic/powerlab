import { generateID } from '$lib/utils/uuid';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
	id: string;
	type: ToastType;
	message: string;
	duration: number;
}

let toasts = $state<Toast[]>([]);

function add(type: ToastType, message: string, duration = 4000) {
	// Deduplication: prevent identical toasts from stacking
	if (toasts.some((t) => t.type === type && t.message === message)) {
		return;
	}

	// generateID falls back to crypto.getRandomValues / Math.random
	// when crypto.randomUUID is unavailable. Required for
	// non-secure contexts (http://IP:port) — crypto.randomUUID
	// only exists in secure contexts and threw a TypeError in
	// v0.5.2 after HTTPS was gated by default (#130).
	const id = generateID();
	toasts = [...toasts, { id, type, message, duration }];
	if (duration > 0) {
		setTimeout(() => dismiss(id), duration);
	}
	return id;
}

function dismiss(id: string | undefined) {
	if (!id) return;
	toasts = toasts.filter((t) => t.id !== id);
}

export const toast = {
	get toasts() { return toasts; },
	success: (message: string, duration?: number) => add('success', message, duration),
	error: (message: string, duration?: number) => add('error', message, duration),
	warning: (message: string, duration?: number) => add('warning', message, duration),
	info: (message: string, duration?: number) => add('info', message, duration),
	dismiss
};
