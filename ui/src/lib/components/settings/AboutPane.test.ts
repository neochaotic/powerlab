import { describe, it, expect, vi } from 'vitest';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = '0.5.13-test';

const mockStorage: Record<string, string> = {};
global.localStorage = {
	getItem: vi.fn((key: string) => mockStorage[key] ?? null),
	setItem: vi.fn((key: string, value: string) => {
		mockStorage[key] = value;
	}),
	removeItem: vi.fn((key: string) => {
		delete mockStorage[key];
	}),
	clear: vi.fn(() => {
		for (const k in mockStorage) delete mockStorage[k];
	}),
	length: 0,
	key: (i: number) => Object.keys(mockStorage)[i] ?? null
} as Storage;

vi.mock('$lib/stores/updater.svelte', () => ({
	updaterStore: {
		releaseInfo: null,
		checking: false,
		error: null,
		check: vi.fn(),
		downloadProgress: 0,
		failureState: {
			consecutiveFailures: 0,
			lastSuccessTs: null,
			transientFailure: false,
			persistentFailure: false,
			lastCheckedHumanRelative: null
		}
	}
}));

const { render, screen } = await import('@testing-library/svelte');
const { default: AboutPane } = await import('./AboutPane.svelte');

describe('AboutPane', () => {
	it('renders the PowerLab hero block with the stamped version', () => {
		render(AboutPane);
		expect(screen.getByText(/v0\.5\.13-test/)).toBeTruthy();
	});

	it('renders the license badge', () => {
		render(AboutPane);
		expect(screen.getByText('AGPL-3.0')).toBeTruthy();
	});

	it('renders the Pre-release marker', () => {
		render(AboutPane);
		expect(screen.getByText(/Pre-release/i)).toBeTruthy();
	});
});
