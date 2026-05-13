import { describe, it, expect, beforeEach } from 'vitest';
import { useInstallState } from './install-state.svelte';
import type { ComposeAppStoreInfo } from '$lib/api/apps';

const fakeStoreInfo = {
	store_app_id: 'enclosed',
	title: { en_us: 'Enclosed' },
	icon: 'https://example.com/icon.svg',
	port_map: '8788',
} as unknown as ComposeAppStoreInfo;

describe('useInstallState', () => {
	beforeEach(() => {
		// Clear any leftover state from prior tests.
		const s = useInstallState();
		for (const e of s.all) s.finish(e.id);
	});

	it('starts empty', () => {
		const s = useInstallState();
		expect(s.all).toEqual([]);
		expect(s.get('anything')).toBeNull();
	});

	it('start() registers a new install entry with installing phase + zero progress', () => {
		const s = useInstallState();
		s.start('enclosed', fakeStoreInfo);
		const entry = s.get('enclosed');
		expect(entry).not.toBeNull();
		expect(entry!.phase).toBe('installing');
		expect(entry!.progress).toBe(0);
		expect(entry!.currentPhase).toBeNull();
		expect(entry!.logs).toBe('');
		expect(entry!.storeInfo.store_app_id).toBe('enclosed');
		expect(entry!.startedAt).toBeGreaterThan(0);
	});

	it('all returns every tracked entry', () => {
		const s = useInstallState();
		s.start('enclosed', fakeStoreInfo);
		s.start('agent-zero', { ...fakeStoreInfo, store_app_id: 'agent-zero' } as ComposeAppStoreInfo);
		expect(s.all).toHaveLength(2);
		expect(s.all.map(e => e.id).sort()).toEqual(['agent-zero', 'enclosed']);
	});

	it('update() merges patch into tracked entry, preserves untouched fields', () => {
		const s = useInstallState();
		s.start('enclosed', fakeStoreInfo);
		const ok = s.update('enclosed', { phase: 'starting', progress: 0.4 });
		expect(ok).toBe(true);
		const entry = s.get('enclosed');
		expect(entry!.phase).toBe('starting');
		expect(entry!.progress).toBe(0.4);
		// storeInfo + startedAt preserved
		expect(entry!.storeInfo.store_app_id).toBe('enclosed');
	});

	it("update() returns false when the install isn't tracked (race-safe)", () => {
		const s = useInstallState();
		const ok = s.update('not-installing', { progress: 1 });
		expect(ok).toBe(false);
	});

	it('finish() removes the install from tracking', () => {
		const s = useInstallState();
		s.start('enclosed', fakeStoreInfo);
		expect(s.get('enclosed')).not.toBeNull();
		s.finish('enclosed');
		expect(s.get('enclosed')).toBeNull();
		expect(s.all).toEqual([]);
	});

	it('finish() on unknown id is a no-op (does not throw)', () => {
		const s = useInstallState();
		expect(() => s.finish('not-installing')).not.toThrow();
	});

	// Sprint 13.2.2 contract: install state persists across "close
	// modal without canceling" so the launchpad can render a ghost
	// tile. Test the state survives multiple updates.
	it('tracks an install across multiple phase transitions', () => {
		const s = useInstallState();
		s.start('enclosed', fakeStoreInfo);
		s.update('enclosed', { phase: 'starting', logs: '[hash] Pulling\n' });
		s.update('enclosed', {
			currentPhase: { step: 2, total: 5, label: 'Pulling images' },
			progress: 0.4,
		});
		s.update('enclosed', {
			currentPhase: { step: 5, total: 5, label: 'Done' },
			progress: 1,
		});
		const entry = s.get('enclosed');
		expect(entry!.phase).toBe('starting');
		expect(entry!.progress).toBe(1);
		expect(entry!.currentPhase?.step).toBe(5);
		expect(entry!.logs).toBe('[hash] Pulling\n');
	});
});
