import { describe, it, expect } from 'vitest';
import { detectAppSource, appSourceLabel, appSourceUpstreamURL } from './app-source';

describe('detectAppSource', () => {
	it('honors explicit source.catalog=umbrel-apps', () => {
		expect(detectAppSource({ source: { catalog: 'umbrel-apps' } })).toBe('umbrel');
	});

	it('normalises explicit source.catalog with trailing variants', () => {
		expect(detectAppSource({ source: { catalog: 'umbrel' } })).toBe('umbrel');
		expect(detectAppSource({ source: { catalog: 'casaos' } })).toBe('casaos');
		expect(detectAppSource({ source: { catalog: 'big-bear-casaos' } })).toBe('big-bear');
	});

	it('returns "store" for unknown explicit source value', () => {
		expect(detectAppSource({ source: { catalog: 'unknown-provider' } })).toBe('store');
	});

	it('falls back to icon heuristic when no explicit source — umbrel', () => {
		expect(detectAppSource({
			icon: 'https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg'
		})).toBe('umbrel');
	});

	it('falls back to icon heuristic — casaos', () => {
		expect(detectAppSource({
			icon: 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@main/Apps/AdGuardHome/icon.png'
		})).toBe('casaos');
	});

	it('falls back to icon heuristic — big-bear', () => {
		expect(detectAppSource({
			icon: 'https://cdn.jsdelivr.net/gh/bigbeartechworld/big-bear-casaos/.../icon.png'
		})).toBe('big-bear');
	});

	it('returns "store" when no source and no recognized icon host', () => {
		expect(detectAppSource({ icon: 'https://example.com/icon.png' })).toBe('store');
	});

	it('returns "store" when icon is missing entirely', () => {
		expect(detectAppSource({})).toBe('store');
	});

	it('returns "store" for null/undefined input', () => {
		expect(detectAppSource(null)).toBe('store');
		expect(detectAppSource(undefined)).toBe('store');
	});

	it('explicit source.catalog wins over icon URL', () => {
		// Edge case: an Umbrel-synced app whose icon URL happens to
		// hit the casaos heuristic (shouldn't happen in practice).
		// Backend's explicit field must take precedence.
		expect(detectAppSource({
			source: { catalog: 'umbrel-apps' },
			icon: 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore/icon.png'
		})).toBe('umbrel');
	});
});

describe('appSourceLabel', () => {
	it('returns the canonical short label', () => {
		expect(appSourceLabel('umbrel')).toBe('umbrel');
		expect(appSourceLabel('casaos')).toBe('casaos');
		expect(appSourceLabel('big-bear')).toBe('big-bear');
		expect(appSourceLabel('store')).toBe('store');
	});
});

describe('appSourceUpstreamURL', () => {
	it('prefers explicit source.upstream_repo when present', () => {
		expect(appSourceUpstreamURL({
			source: { catalog: 'umbrel-apps', upstream_repo: 'https://github.com/getumbrel/umbrel-apps' }
		})).toBe('https://github.com/getumbrel/umbrel-apps');
	});

	it('falls back to well-known URL by detected source', () => {
		expect(appSourceUpstreamURL({
			icon: 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore/icon.png'
		})).toBe('https://github.com/IceWhaleTech/CasaOS-AppStore');
	});

	it('returns null for generic "store" with no upstream pointer', () => {
		expect(appSourceUpstreamURL({ icon: 'https://example.com/x.png' })).toBeNull();
	});

	it('returns null for null/undefined input', () => {
		expect(appSourceUpstreamURL(null)).toBeNull();
		expect(appSourceUpstreamURL(undefined)).toBeNull();
	});
});
