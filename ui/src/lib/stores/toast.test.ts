import { describe, it, expect, beforeEach } from 'vitest';
import { toast } from './toast.svelte';

// Reset all toasts before each test (singleton state bleeds between tests)
beforeEach(() => {
	[...toast.toasts].forEach(t => toast.dismiss(t.id));
});

describe('Toast Store', () => {
	it('toast.success adds a toast with type success', () => {
		toast.success('Deploy completed');
		expect(toast.toasts).toHaveLength(1);
		expect(toast.toasts[0].type).toBe('success');
		expect(toast.toasts[0].message).toBe('Deploy completed');
	});

	it('toast.error adds a toast with type error', () => {
		toast.error('Connection failed');
		expect(toast.toasts[0].type).toBe('error');
		expect(toast.toasts[0].message).toBe('Connection failed');
	});

	it('toast.warning adds a toast with type warning', () => {
		toast.warning('High CPU usage');
		expect(toast.toasts[0].type).toBe('warning');
	});

	it('toast.info adds a toast with type info', () => {
		toast.info('Polling started');
		expect(toast.toasts[0].type).toBe('info');
	});

	it('toast.dismiss removes only the targeted toast', () => {
		const id1 = toast.success('First');
		const id2 = toast.error('Second');
		expect(toast.toasts).toHaveLength(2);

		toast.dismiss(id1);

		expect(toast.toasts).toHaveLength(1);
		expect(toast.toasts[0].message).toBe('Second');
		expect(toast.toasts[0].id).toBe(id2);
	});

	it('toast.dismiss ignores unknown ids (no crash)', () => {
		toast.success('Hello');
		expect(() => toast.dismiss('non-existent-id')).not.toThrow();
		expect(toast.toasts).toHaveLength(1);
	});

	it('multiple toasts stack independently', () => {
		toast.success('A');
		toast.error('B');
		toast.warning('C');
		expect(toast.toasts).toHaveLength(3);
		expect(toast.toasts.map(t => t.type)).toEqual(['success', 'error', 'warning']);
	});

	it('each toast gets a unique id', () => {
		toast.success('One');
		toast.success('Two');
		const ids = toast.toasts.map(t => t.id);
		expect(ids[0]).not.toBe(ids[1]);
	});

	it('default duration is 4000ms', () => {
		toast.info('Timed');
		expect(toast.toasts[0].duration).toBe(4000);
	});

	it('custom duration is respected', () => {
		toast.error('Sticky', 0);
		expect(toast.toasts[0].duration).toBe(0);
	});
});
