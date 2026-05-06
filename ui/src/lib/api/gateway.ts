/**
 * Gateway management API.
 *
 * Wraps the gateway's `/v1/gateway/*` endpoints, which let the user
 * inspect and reconfigure the gateway itself (port, listener) from the
 * UI. The gateway's port-change flow is unique: changing it terminates
 * the very HTTP connection that delivered the request, so callers MUST
 * coordinate the redirect themselves.
 *
 * Backend: see `backend/gateway/route/management_route.go`.
 */

import { api } from './client';

/** Standard CasaOS-shaped envelope returned by the gateway routes. */
type Envelope<T> = {
	success: number;
	message: string;
	data?: T;
};

/**
 * Returns the port the gateway is currently listening on, as a string.
 * The wire format is a string because the underlying config field is
 * stringly typed (`gateway.Port` from the INI file); we convert at the
 * call site rather than masking the API surface.
 */
export async function getGatewayPort(): Promise<string> {
	const res = await api.get<Envelope<string>>('/v1/gateway/port');
	return res.data ?? '';
}

/**
 * Persist a new gateway port. The backend:
 *
 *   1. Validates the value is in [1, 65535] (see
 *      `service.validateGatewayPort` in the gateway package).
 *   2. Re-binds the listener on the new port.
 *   3. Writes `Port = <newport>` to /etc/powerlab/gateway.ini.
 *
 * The HTTP response races the listener swap. Successful responses
 * usually arrive on the OLD socket; any subsequent request from the
 * caller MUST go to the new port. The Settings UI is responsible for
 * showing a countdown + redirecting `window.location.href` to
 * `<host>:<newport>` after the call returns.
 *
 * Throws if the backend rejects the port (range violation, in use,
 * permission denied) — caller should surface the message in a toast.
 */
export async function setGatewayPort(port: number): Promise<void> {
	if (!Number.isInteger(port) || port < 1 || port > 65535) {
		// Pre-validate client-side so we surface a friendlier error
		// without a network round-trip.
		throw new Error(`port ${port} is out of range — must be 1..65535`);
	}
	await api.put<Envelope<void>>('/v1/gateway/port', { port: String(port) });
}
