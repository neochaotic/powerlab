import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { detectOS, getCurrentOS } from './os';

describe('detectOS', () => {
	it('detects iOS', () => {
		const iosUA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1';
		expect(detectOS(iosUA)).toBe('ios');
	});

	it('detects macOS', () => {
		const macUA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36';
		expect(detectOS(macUA)).toBe('macos');
	});

	it('detects Android', () => {
		const androidUA = 'Mozilla/5.0 (Linux; Android 13; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Mobile Safari/537.36';
		expect(detectOS(androidUA)).toBe('android');
	});

	it('detects Windows', () => {
		const winUA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36';
		expect(detectOS(winUA)).toBe('windows');
	});

	it('detects Linux', () => {
		const linuxUA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36';
		expect(detectOS(linuxUA)).toBe('linux');
	});

	it('returns unknown for garbage', () => {
		expect(detectOS('nothing-burger')).toBe('unknown');
	});

	it('returns unknown for empty string', () => {
		expect(detectOS('')).toBe('unknown');
	});

	it('treats Macintosh UA with maxTouchPoints > 2 as iOS (iPadOS 13+)', () => {
		const originalMaxTouchPoints = Object.getOwnPropertyDescriptor(
			Navigator.prototype,
			'maxTouchPoints'
		);
		Object.defineProperty(navigator, 'maxTouchPoints', { value: 5, configurable: true });
		try {
			const macUA =
				'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Safari/537.36';
			expect(detectOS(macUA)).toBe('ios');
		} finally {
			if (originalMaxTouchPoints) {
				Object.defineProperty(Navigator.prototype, 'maxTouchPoints', originalMaxTouchPoints);
			} else {
				Object.defineProperty(navigator, 'maxTouchPoints', { value: 0, configurable: true });
			}
		}
	});
});

describe('getCurrentOS', () => {
	it('delegates to detectOS with navigator.userAgent', () => {
		const original = navigator.userAgent;
		Object.defineProperty(navigator, 'userAgent', {
			value:
				'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0 Safari/537.36',
			configurable: true
		});
		try {
			expect(getCurrentOS()).toBe('linux');
		} finally {
			Object.defineProperty(navigator, 'userAgent', { value: original, configurable: true });
		}
	});
});
