export type OS = 'ios' | 'macos' | 'android' | 'windows' | 'linux' | 'unknown';

/**
 * Detects the Operating System from the User Agent string.
 * @param ua The navigator.userAgent string.
 * @returns The detected OS name.
 */
export function detectOS(ua: string): OS {
	const lowerUA = ua.toLowerCase();

	if (/ipad|iphone|ipod/.test(lowerUA)) {
		return 'ios';
	}

	if (lowerUA.includes('macintosh') || lowerUA.includes('mac os x')) {
		// iPadOS 13+ can report as Macintosh
		if (navigator.maxTouchPoints && navigator.maxTouchPoints > 2) {
			return 'ios';
		}
		return 'macos';
	}

	if (lowerUA.includes('android')) {
		return 'android';
	}

	if (lowerUA.includes('windows')) {
		return 'windows';
	}

	if (lowerUA.includes('linux')) {
		return 'linux';
	}

	return 'unknown';
}

/**
 * Returns the current OS of the client.
 */
export function getCurrentOS(): OS {
	if (typeof navigator === 'undefined') return 'unknown';
	return detectOS(navigator.userAgent);
}
