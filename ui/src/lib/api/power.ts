/**
 * Power-actions API client (#260, Sprint 23).
 *
 * Wraps the four endpoints shipped in backend/core/route/v1/power.go
 * that the Settings → Power pane drives. Service names are validated
 * server-side against a hardcoded allow-list (PowerLabServices); this
 * client doesn't replicate the list so it can't drift.
 *
 * Endpoint map:
 *
 *   GET  /v1/sys/services                   → list PowerLab units + state
 *   POST /v1/sys/services/{name}/restart    → restart a whitelisted unit
 *   POST /v1/sys/host/reboot                → systemctl reboot (requires confirm)
 *   POST /v1/sys/host/shutdown              → systemctl poweroff (requires confirm)
 *
 * Memory feedback_security_is_priority: host-level ops include the
 * {"confirm": true} body the backend requires; a caller that wants
 * UI-level "are you sure?" must still gate the call on its own.
 */

import { api } from './client';

export interface ServiceState {
	/** PowerLab systemd unit name (e.g. "powerlab-gateway") */
	name: string;
	/** systemctl ActiveState: active | inactive | failed | activating | deactivating | unknown */
	active_state: string;
	/** systemctl SubState: running | dead | exited | start-pre | ... */
	sub_state?: string;
	/** PID when running; absent or "" when stopped */
	pid?: string;
}

interface Envelope<T> {
	data: T;
	message?: string;
}

/**
 * GET the current state of every PowerLab systemd unit. Returns an
 * empty array if the backend sends `data: null` (rare but defensive).
 */
export async function listPowerLabServices(): Promise<ServiceState[]> {
	const res = await api.get<Envelope<ServiceState[] | null>>('/v1/sys/services');
	return res.data ?? [];
}

/**
 * Restart a single PowerLab unit. Server validates the name against
 * its allow-list; an unknown name returns 400. The client url-encodes
 * the segment defensively even though current backend behaviour
 * rejects anything with characters outside the curated 6 names.
 */
export async function restartPowerLabService(name: string): Promise<void> {
	await api.post(`/v1/sys/services/${encodeURIComponent(name)}/restart`);
}

/**
 * Trigger `systemctl reboot` on the host. Backend requires
 * `{"confirm": true}` body; a stray empty POST returns 400. Callers
 * MUST still surface their own "are you sure?" UI gate — the backend
 * confirm is the second line of defence, not the first.
 */
export async function rebootHost(): Promise<void> {
	await api.post('/v1/sys/host/reboot', { confirm: true });
}

/**
 * Trigger `systemctl poweroff` on the host. Same caveats as rebootHost.
 */
export async function shutdownHost(): Promise<void> {
	await api.post('/v1/sys/host/shutdown', { confirm: true });
}
