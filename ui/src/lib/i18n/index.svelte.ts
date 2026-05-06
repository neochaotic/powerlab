/**
 * PowerLab i18n — Minimal translation system.
 *
 * V1 is 100% English. Structure is ready for future locales.
 * Usage: import { t } from '$lib/i18n'; t('dashboard.title')
 */

type Translations = Record<string, Record<string, string>>;

const translations: Translations = {
	en: {
		// App
		'app.name': 'PowerLab',
		'app.tagline': 'Headless OS Management Panel',

		// Nav
		'nav.dashboard': 'Dashboard',
		'nav.files': 'Files',
		'nav.apps': 'Apps',
		'nav.storage': 'Storage',
		'nav.settings': 'Settings',

		// Dashboard
		'dashboard.title': 'Dashboard',
		'dashboard.cpu': 'CPU Usage',
		'dashboard.memory': 'Memory',
		'dashboard.storage': 'Storage',
		'dashboard.network': 'Network',
		'dashboard.services': 'Services',

		// Actions
		'action.start': 'Start',
		'action.stop': 'Stop',
		'action.restart': 'Restart',
		'action.install': 'Install',
		'action.uninstall': 'Uninstall',
		'action.update': 'Update',
		'action.save': 'Save',
		'action.cancel': 'Cancel',
		'action.delete': 'Delete',
		'action.confirm': 'Confirm',

		// Status
		'status.online': 'Online',
		'status.offline': 'Offline',
		'status.running': 'Running',
		'status.stopped': 'Stopped',
		'status.loading': 'Loading...',
		'status.error': 'Error',

		// Files
		'files.title': 'File Manager',
		'files.upload': 'Upload',
		'files.download': 'Download',
		'files.rename': 'Rename',
		'files.copy': 'Copy',
		'files.cut': 'Cut',
		'files.paste': 'Paste',
		'files.newFolder': 'New Folder',

		// Errors
		'error.network': 'Unable to reach the server',
		'error.unauthorized': 'Session expired. Please log in again.',
		'error.notFound': 'Resource not found',
		'error.generic': 'Something went wrong'
	}
};

let currentLocale = $state('en');

export function setLocale(locale: string) {
	if (translations[locale]) {
		currentLocale = locale;
	}
}

export function t(key: string): string {
	return translations[currentLocale]?.[key] ?? key;
}

export function getLocale(): string {
	return currentLocale;
}
