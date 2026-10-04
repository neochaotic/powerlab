import yaml from 'js-yaml';

/**
 * Pre-install analysis of a compose document: which host ports it will
 * publish (so the install flow can probe them for conflicts) and which
 * compatibility / risk warnings to show the user.
 *
 * Extracted from `routes/apps/+page.svelte` (which parsed the compose
 * inline, typed `as any`). The inline version read the short-syntax
 * port "127.0.0.1:8080:80" as host port 127 because it took
 * `split(':')[0]`. This module parses compose port syntax properly.
 *
 * Port-collection rules (documented decisions):
 *   - "80" (container port only) publishes on an ephemeral host port
 *     chosen by Docker, so it can never conflict: NOT collected.
 *     Bare numbers (`- 80` in YAML) are treated the same way.
 *   - "8080:80", "127.0.0.1:8080:80", "[::1]:8080:80" → 8080.
 *   - "127.0.0.1::80" (IP with empty host port) → ephemeral, skipped.
 *   - "/tcp" / "/udp" protocol suffixes are ignored.
 *   - Host ranges ("8000-8002:8000-8002") → EVERY port in the range is
 *     collected, since each one can conflict. Ranges wider than
 *     MAX_RANGE_EXPANSION are skipped rather than probed port-by-port.
 *   - Env interpolation is best-effort: "${PORT:-8080}" / "${PORT-8080}"
 *     resolve to their default (8080). A variable with no default
 *     ("${PORT}", "$PORT") is unknowable client-side and is skipped.
 *     Never throws.
 *   - Long syntax `{ published, target }`: `published` may be a number
 *     or a string (including a range or an interpolation).
 * Ports are deduplicated across all services, in first-seen order.
 */

export const MAX_RANGE_EXPANSION = 1000;

export const WARNING_HOST_NETWORK =
	"This app needs network_mode: host which doesn't work on Docker Desktop (macOS/Windows)";
export const WARNING_PRIVILEGED = 'This app requires privileged mode (Critical Risk)';
export const WARNING_CAP_ADD = 'Needs Linux kernel capabilities (cap_add)';
export const WARNING_KERNEL_DEVICES = 'Needs Linux kernel devices (/dev, /proc, /sys)';

export interface ComposeAnalysis {
	/** Published host ports across all services, deduplicated. */
	requestedPorts: number[];
	/** Human-readable compatibility / risk warnings, in service order. */
	compatibilityWarnings: string[];
}

interface ComposeService {
	network_mode?: unknown;
	privileged?: unknown;
	cap_add?: unknown;
	volumes?: unknown;
	ports?: unknown;
}

function isRecord(v: unknown): v is Record<string, unknown> {
	return typeof v === 'object' && v !== null && !Array.isArray(v);
}

/**
 * Replace `${VAR:-default}` / `${VAR-default}` with `default`. Any
 * remaining `${VAR}` / `$VAR` makes the value unresolvable → null.
 */
export function resolveInterpolation(value: string): string | null {
	const resolved = value.replace(/\$\{[A-Za-z_][A-Za-z0-9_]*:?-([^}]*)\}/g, '$1');
	if (resolved.includes('$')) return null;
	return resolved;
}

/** Parse "8080" or "8000-8002" into the list of ports it names. */
function parsePortSpec(spec: string): number[] {
	const s = spec.trim();
	if (s === '') return [];
	const range = /^(\d+)-(\d+)$/.exec(s);
	if (range) {
		const start = Number(range[1]);
		const end = Number(range[2]);
		if (!isValidPort(start) || !isValidPort(end) || end < start) return [];
		if (end - start + 1 > MAX_RANGE_EXPANSION) return [];
		const out: number[] = [];
		for (let p = start; p <= end; p++) out.push(p);
		return out;
	}
	if (!/^\d+$/.test(s)) return [];
	const n = Number(s);
	return isValidPort(n) ? [n] : [];
}

function isValidPort(n: number): boolean {
	return Number.isInteger(n) && n > 0 && n <= 65535;
}

/**
 * Host ports published by one short-syntax entry such as
 * "127.0.0.1:8080:80/udp". Returns [] when the entry publishes on an
 * ephemeral port or cannot be resolved.
 */
export function parseShortPortSyntax(entry: string): number[] {
	const resolved = resolveInterpolation(entry.trim());
	if (resolved === null) return [];
	// Strip protocol suffix.
	const noProto = resolved.replace(/\/(tcp|udp|sctp)$/i, '');

	let rest = noProto;
	// Bracketed IPv6 host IP: "[::1]:8080:80".
	if (rest.startsWith('[')) {
		const close = rest.indexOf(']');
		if (close === -1 || rest[close + 1] !== ':') return [];
		rest = rest.slice(close + 2);
		const parts = rest.split(':');
		// After the IP we expect "host:container" (host may be empty).
		return parts.length === 2 ? parsePortSpec(parts[0]) : [];
	}

	const parts = rest.split(':');
	switch (parts.length) {
		case 1:
			return []; // container port only → ephemeral host port
		case 2:
			return parsePortSpec(parts[0]); // host:container
		case 3:
			return parsePortSpec(parts[1]); // ip:host:container
		default:
			return []; // unbracketed IPv6 or garbage — skip rather than guess
	}
}

/** Host ports published by one `ports:` entry (short or long syntax). */
export function publishedHostPorts(entry: unknown): number[] {
	if (typeof entry === 'string') return parseShortPortSyntax(entry);
	if (isRecord(entry)) {
		const published = entry.published;
		if (typeof published === 'number') return isValidPort(published) ? [published] : [];
		if (typeof published === 'string') {
			const resolved = resolveInterpolation(published);
			return resolved === null ? [] : parsePortSpec(resolved);
		}
	}
	// Bare numbers are container-only; anything else is unparseable.
	return [];
}

/** Analyse an already-parsed compose document. Never throws. */
export function analyzeCompose(doc: unknown): ComposeAnalysis {
	const requestedPorts: number[] = [];
	const compatibilityWarnings: string[] = [];
	if (!isRecord(doc) || !isRecord(doc.services)) {
		return { requestedPorts, compatibilityWarnings };
	}

	for (const raw of Object.values(doc.services)) {
		if (!isRecord(raw)) continue;
		const svc = raw as ComposeService;

		if (svc.network_mode === 'host') {
			compatibilityWarnings.push(WARNING_HOST_NETWORK);
		}
		if (svc.privileged === true) {
			compatibilityWarnings.push(WARNING_PRIVILEGED);
		}
		if (Array.isArray(svc.cap_add)) {
			if (svc.cap_add.some((c) => typeof c === 'string' && (c.includes('ADMIN') || c.includes('NET')))) {
				compatibilityWarnings.push(WARNING_CAP_ADD);
			}
		}
		if (Array.isArray(svc.volumes)) {
			const touchesKernel = svc.volumes.some((v) => {
				const path = typeof v === 'string' ? v : isRecord(v) ? v.source : undefined;
				return (
					typeof path === 'string' &&
					(path.startsWith('/dev/') || path.startsWith('/proc/') || path.startsWith('/sys/'))
				);
			});
			if (touchesKernel) compatibilityWarnings.push(WARNING_KERNEL_DEVICES);
		}
		if (Array.isArray(svc.ports)) {
			for (const p of svc.ports) {
				for (const n of publishedHostPorts(p)) {
					if (!requestedPorts.includes(n)) requestedPorts.push(n);
				}
			}
		}
	}

	return { requestedPorts, compatibilityWarnings };
}

/** Parse YAML text and analyse it. Throws only if the YAML is invalid. */
export function analyzeComposeYaml(yamlText: string): ComposeAnalysis {
	return analyzeCompose(yaml.load(yamlText));
}
