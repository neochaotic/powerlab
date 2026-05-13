import { describe, it, expect } from 'vitest';
import { buildAppURL } from './app-url';

describe('buildAppURL', () => {
	it('default http scheme when scheme is missing', () => {
		expect(buildAppURL('192.168.18.86', { port_map: '8788' }))
			.toBe('http://192.168.18.86:8788');
	});

	it('default http scheme when scheme is the string "http"', () => {
		expect(buildAppURL('host.local', { port_map: '8080', scheme: 'http' }))
			.toBe('http://host.local:8080');
	});

	// Regression for the openclaw + 5 sibling CasaOS apps that
	// silently failed: scheme=https in catalog, launchpad always
	// hard-coded http://, container actually serves HTTPS, blank
	// tab on click. #244.
	it('respects scheme: https when set (CasaOS-curated apps)', () => {
		expect(buildAppURL('192.168.18.86', { port_map: '24190', scheme: 'https' }))
			.toBe('https://192.168.18.86:24190');
	});

	// Regression for openclaw specifically: CasaOS's legacy token-auth
	// proxy left ?token=casaos in the catalog metadata. PowerLab
	// doesn't replicate that proxy; passing the token through just
	// confuses the underlying app. Strip it. Other index: values
	// (real paths or anchors) MUST be preserved.
	it('strips ?token=casaos legacy auth-proxy query (openclaw-class)', () => {
		expect(buildAppURL('192.168.18.86', {
			port_map: '24190',
			scheme: 'https',
			index: '?token=casaos',
		}))
			.toBe('https://192.168.18.86:24190');
	});

	it('preserves non-legacy index path/anchor', () => {
		expect(buildAppURL('host.local', {
			port_map: '3000',
			index: '/admin',
		}))
			.toBe('http://host.local:3000/admin');
	});

	it('preserves index that looks like a query but is not the casaos token', () => {
		expect(buildAppURL('host.local', {
			port_map: '3000',
			index: '?lang=en',
		}))
			.toBe('http://host.local:3000?lang=en');
	});

	it('returns null when port_map is missing', () => {
		expect(buildAppURL('host.local', {})).toBeNull();
		expect(buildAppURL('host.local', null)).toBeNull();
		expect(buildAppURL('host.local', undefined)).toBeNull();
	});

	it('returns null when port_map is empty string', () => {
		expect(buildAppURL('host.local', { port_map: '' })).toBeNull();
	});
});
