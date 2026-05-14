// ESLint flat config — see https://eslint.org/docs/latest/use/configure/configuration-files
//
// Sprint 15 #353 — banishes raw `fetch()` calls from stores/routes/components
// so the v0.6.7 → v0.6.10 upgrade-401 bug class cannot return. Every
// authenticated route MUST go through `src/lib/api/client.ts` because the
// api client is what injects the JWT `Authorization` header on every
// request. Raw `fetch()` skips that. The contract regression test in
// `upgradeProgress.test.ts` locks the specific function that broke; this
// ESLint rule extends that protection to every store/route/component that
// might be written next.
//
// Focused scope: this config's SOLE purpose is the raw-fetch ban. We
// deliberately do NOT pull `js.configs.recommended` because that ruleset
// trips ~280 unrelated errors (no-undef on browser globals, no-unused-vars
// in WIP code). A repo-wide lint enable is its own initiative — out of
// scope here.

import svelte from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';
import tsParser from '@typescript-eslint/parser';

const FETCH_BAN_RULE = {
	'no-restricted-syntax': [
		'error',
		{
			selector: "CallExpression[callee.type='Identifier'][callee.name='fetch']",
			message:
				"Use api.get/post/put/delete from $lib/api/client instead of raw fetch(). Raw fetch bypasses JWT Authorization injection — see Sprint 15 #353 / v0.6.10 upgrade-401 postmortem. If this call is to an intentionally public probe path, add it to the allow-list block in eslint.config.js with a justification comment."
		}
	]
};

export default [
	// Default: ban raw fetch() in stores/routes/components (.ts files).
	{
		files: [
			'src/lib/stores/**/*.ts',
			'src/routes/**/*.ts',
			'src/lib/components/**/*.ts',
			'src/lib/api/**/*.ts'
		],
		languageOptions: {
			parser: tsParser,
			parserOptions: { ecmaVersion: 'latest', sourceType: 'module' }
		},
		rules: FETCH_BAN_RULE
	},

	// Same rule for Svelte files in those folders (script block).
	{
		files: [
			'src/lib/stores/**/*.svelte',
			'src/lib/stores/**/*.svelte.ts',
			'src/routes/**/*.svelte',
			'src/lib/components/**/*.svelte'
		],
		languageOptions: {
			parser: svelteParser,
			parserOptions: { parser: tsParser, ecmaVersion: 'latest', sourceType: 'module', extraFileExtensions: ['.svelte'] }
		},
		plugins: { svelte },
		rules: FETCH_BAN_RULE
	},

	// Allow-list — these 5 paths are intentionally raw and MUST stay so.
	// 4 are public probes (pre-login or HTTPS onboarding); 1 is the api
	// client itself (the abstraction that injects Authorization for
	// everyone else). Adding a new file here demands a justification
	// comment so reviewers can challenge whether the claim holds.
	{
		files: [
			// THE api client — by definition this is where the one real
			// raw fetch() lives; api.get/post/put/delete wrap it. Banning
			// fetch here would be banning the implementation of the very
			// abstraction the rule asks everyone else to use.
			'src/lib/api/client.ts',
			// Pre-login version handshake — runs before auth store is ready.
			'src/lib/stores/versionHandshake.svelte.ts',
			// Inside upgradeProgress poll, only for /v1/powerlab/version (public).
			// The POST /v1/powerlab-update/install in this file ALREADY uses
			// api.post per #352 — only the poll fetch is exempt.
			'src/lib/stores/upgradeProgress.svelte.ts',
			// HTTPS onboarding probe — must work BEFORE the user has trusted
			// the CA, hence no auth context yet.
			'src/lib/components/security/TrustStateChecker.svelte',
			// /v1/sys/trust-confirmed DELETE is public-by-design (operator
			// resets the local trust state from the settings page).
			'src/routes/settings/+page.svelte'
		],
		rules: { 'no-restricted-syntax': 'off' }
	},

	// Tests + test infrastructure can use raw fetch — they need to swap
	// global.fetch for mocks (mockFetchSequence, vi.fn(), etc.).
	{
		files: [
			'src/**/*.test.ts',
			'src/**/*.spec.ts',
			'tests/**/*.ts',
			'tests/**/*.spec.ts'
		],
		rules: { 'no-restricted-syntax': 'off' }
	}
];
