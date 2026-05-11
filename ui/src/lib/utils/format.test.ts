import { describe, it, expect } from 'vitest';
import { formatSize, formatPercent } from './format';

describe('formatSize', () => {
	it('returns "0 B" exactly for zero bytes (no NaN from log(0))', () => {
		expect(formatSize(0)).toBe('0 B');
	});

	it('formats bytes below 1 KB with no decimal', () => {
		expect(formatSize(512)).toBe('512 B');
		expect(formatSize(1023)).toBe('1023 B');
	});

	it('formats KB with one decimal', () => {
		expect(formatSize(1024)).toBe('1.0 KB');
		expect(formatSize(1536)).toBe('1.5 KB');
	});

	it('formats MB with one decimal', () => {
		expect(formatSize(1024 * 1024)).toBe('1.0 MB');
		expect(formatSize(2.5 * 1024 * 1024)).toBe('2.5 MB');
	});

	it('formats GB with one decimal', () => {
		expect(formatSize(1024 ** 3)).toBe('1.0 GB');
	});

	it('formats TB with one decimal', () => {
		expect(formatSize(1024 ** 4)).toBe('1.0 TB');
	});
});

describe('formatPercent', () => {
	it('formats integer percent with one decimal', () => {
		expect(formatPercent(50)).toBe('50.0%');
	});

	it('formats fractional percent with one decimal', () => {
		expect(formatPercent(33.456)).toBe('33.5%');
	});

	it('formats zero as 0.0%', () => {
		expect(formatPercent(0)).toBe('0.0%');
	});
});
