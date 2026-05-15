import yaml from 'js-yaml';

/**
 * Pure functions that mutate a compose YAML string by editing a
 * specific field, preserving the rest of the document including:
 *   - long-form vs short-form entries (the bug class behind issue
 *     #332). If a volume / port / device was written in long-form
 *     ({type, source, target}), an edit only modifies the relevant
 *     sub-field; the entry stays long-form.
 *   - comments. js-yaml drops comments on round-trip; that is
 *     accepted (Custom App users have a YAML view to see verbatim
 *     output). The form-mediated edits are committed; the user's
 *     subjective comments are lost. Documented trade-off.
 *
 * Each function takes the current YAML string + new value, returns
 * a new YAML string. They are PURE — no shared state, no mutation
 * of caller's data. The Custom App page wires these as the onChange
 * callbacks of one-way form inputs.
 *
 * The first service key (alphabetical order from js-yaml) is the
 * "active" service the form edits. Single-service custom apps are
 * the dominant case; multi-service edits require the YAML view.
 */

// ─── Read helpers (used by the form's $derived view) ────────────────────────

export interface ComposeView {
	projectName: string;
	serviceName: string;
	image: string;
	containerName: string;
	icon: string;
	webPort: string;
	network: string;
	command: string;
	user: string;
	workingDir: string;
	privileged: boolean;
	memLimitMb: number;
	restart: string;
	ports: Array<{ host: string; container: string; raw: unknown }>;
	volumes: Array<{ host: string; container: string; raw: unknown }>;
	devices: Array<{ host: string; container: string; raw: unknown }>;
	env: Array<{ key: string; value: string }>;
	labels: Array<{ key: string; value: string }>;
}

export const EMPTY_VIEW: ComposeView = {
	projectName: '',
	serviceName: '',
	image: '',
	containerName: '',
	icon: '',
	webPort: '',
	network: 'bridge',
	command: '',
	user: '',
	workingDir: '',
	privileged: false,
	memLimitMb: 512,
	restart: 'always',
	ports: [],
	volumes: [],
	devices: [],
	env: [],
	labels: []
};

/**
 * Parse a YAML string into a read-only ComposeView. Resilient to
 * partial / invalid YAML — returns an empty view rather than
 * throwing; the caller can render placeholders. The `raw` field on
 * port/volume/device entries preserves the original shape so the
 * write helpers can re-emit long-form correctly.
 */
export function viewFromYaml(yamlText: string): ComposeView {
	const v: ComposeView = { ...EMPTY_VIEW, ports: [], volumes: [], devices: [], env: [], labels: [] };
	if (!yamlText.trim()) return v;
	let parsed: Record<string, unknown> | null;
	try {
		parsed = yaml.load(yamlText) as Record<string, unknown> | null;
	} catch {
		return v;
	}
	if (!parsed || typeof parsed !== 'object') return v;
	if (typeof parsed.name === 'string') v.projectName = parsed.name;
	if (typeof parsed['x-icon'] === 'string') v.icon = parsed['x-icon'] as string;
	const services = parsed.services as Record<string, Record<string, unknown>> | undefined;
	if (!services) return v;
	const firstKey = Object.keys(services)[0];
	if (!firstKey) return v;
	v.serviceName = firstKey;
	const svc = services[firstKey];
	if (typeof svc.image === 'string') v.image = svc.image;
	if (typeof svc.container_name === 'string') v.containerName = svc.container_name;
	if (typeof svc.network_mode === 'string') v.network = svc.network_mode;
	if (typeof svc.command === 'string') v.command = svc.command;
	else if (Array.isArray(svc.command)) v.command = (svc.command as string[]).join(' ');
	if (typeof svc.user === 'string') v.user = svc.user;
	if (typeof svc.working_dir === 'string') v.workingDir = svc.working_dir;
	if (svc.privileged === true) v.privileged = true;
	if (typeof svc.restart === 'string') v.restart = svc.restart;

	const deploy = svc.deploy as { resources?: { limits?: { memory?: string } } } | undefined;
	const memLimit = deploy?.resources?.limits?.memory ?? (svc.mem_limit as string | undefined);
	if (memLimit) v.memLimitMb = parseInt(String(memLimit), 10) || 512;

	const xExt = (parsed['x-powerlab'] || parsed['x-casaos'] || parsed['x-web']) as
		| Record<string, unknown>
		| undefined;
	if (xExt && typeof xExt.port_map === 'string') v.webPort = xExt.port_map;

	if (Array.isArray(svc.ports)) {
		v.ports = svc.ports.map((p: unknown) => {
			if (typeof p === 'string') {
				const [host, container] = p.split(':');
				return { host: host ?? '', container: container ?? host ?? '', raw: p };
			}
			if (p && typeof p === 'object') {
				const o = p as { published?: string | number; target?: string | number };
				return {
					host: String(o.published ?? ''),
					container: String(o.target ?? ''),
					raw: p
				};
			}
			return { host: '', container: '', raw: p };
		});
	}
	if (Array.isArray(svc.volumes)) {
		v.volumes = svc.volumes.map((vol: unknown) => {
			if (typeof vol === 'string') {
				const [host, container] = vol.split(':');
				return { host: host ?? '', container: container ?? '', raw: vol };
			}
			if (vol && typeof vol === 'object') {
				const o = vol as { source?: string; target?: string };
				return {
					host: o.source ?? '',
					container: o.target ?? '',
					raw: vol
				};
			}
			return { host: '', container: '', raw: vol };
		});
	}
	if (Array.isArray(svc.devices)) {
		v.devices = svc.devices.map((d: unknown) => {
			if (typeof d === 'string') {
				const [host, container] = d.split(':');
				return { host: host ?? '', container: container ?? host ?? '', raw: d };
			}
			return { host: '', container: '', raw: d };
		});
	}
	if (Array.isArray(svc.environment)) {
		v.env = (svc.environment as string[]).map((e) => {
			const idx = e.indexOf('=');
			return idx === -1 ? { key: e, value: '' } : { key: e.slice(0, idx), value: e.slice(idx + 1) };
		});
	} else if (svc.environment && typeof svc.environment === 'object') {
		v.env = Object.entries(svc.environment as Record<string, unknown>).map(([k, val]) => ({
			key: k,
			value: String(val)
		}));
	}
	if (svc.labels && typeof svc.labels === 'object' && !Array.isArray(svc.labels)) {
		v.labels = Object.entries(svc.labels as Record<string, unknown>).map(([k, val]) => ({
			key: k,
			value: String(val)
		}));
	} else if (Array.isArray(svc.labels)) {
		v.labels = (svc.labels as string[]).map((s) => {
			const idx = s.indexOf('=');
			return idx === -1 ? { key: s, value: '' } : { key: s.slice(0, idx), value: s.slice(idx + 1) };
		});
	}
	return v;
}

// ─── Mutation helpers (one-way write path) ──────────────────────────────────
//
// Each function takes the current YAML and the new value, returns
// the new YAML string. They walk the parsed document, mutate the
// minimum needed, and re-serialise. Long-form vs short-form is
// preserved per-entry — see preserveShape helpers below.

interface Doc {
	name?: string;
	'x-icon'?: string;
	'x-powerlab'?: Record<string, unknown>;
	services?: Record<string, Service>;
	[k: string]: unknown;
}
interface Service {
	image?: string;
	container_name?: string;
	network_mode?: string;
	command?: string | string[];
	user?: string;
	working_dir?: string;
	privileged?: boolean;
	restart?: string;
	ports?: unknown[];
	volumes?: unknown[];
	devices?: unknown[];
	environment?: unknown;
	labels?: unknown;
	deploy?: { resources?: { limits?: { memory?: string } } };
	mem_limit?: string;
	[k: string]: unknown;
}

function loadDoc(yamlText: string): { doc: Doc; firstSvcKey: string | null } {
	let doc: Doc = {};
	try {
		const parsed = yaml.load(yamlText) as Doc | null;
		if (parsed && typeof parsed === 'object') doc = parsed;
	} catch {
		// Bad yaml — return empty doc; the caller's mutation seeds a
		// minimal valid shape. This is the "first interaction with an
		// invalid YAML" path; rare.
	}
	if (!doc.services || typeof doc.services !== 'object') {
		doc.services = {};
	}
	const firstSvcKey = Object.keys(doc.services)[0] ?? null;
	return { doc, firstSvcKey };
}

function dumpDoc(doc: Doc): string {
	return yaml.dump(doc, { indent: 2, lineWidth: -1 });
}

function ensureService(doc: Doc, key: string): Service {
	if (!doc.services) doc.services = {};
	if (!doc.services[key]) doc.services[key] = {};
	return doc.services[key];
}

// ─── Scalar setters ─────────────────────────────────────────────────────────

export function setProjectName(yamlText: string, name: string): string {
	const { doc } = loadDoc(yamlText);
	doc.name = name;
	return dumpDoc(doc);
}

export function setServiceName(yamlText: string, newKey: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	if (!firstSvcKey || !doc.services) {
		// Bootstrap: create a service.
		doc.services = { [newKey]: {} };
		return dumpDoc(doc);
	}
	if (firstSvcKey === newKey) return yamlText;
	// Rebuild services object preserving order.
	const newServices: Record<string, Service> = {};
	for (const [k, v] of Object.entries(doc.services)) {
		newServices[k === firstSvcKey ? newKey : k] = v;
	}
	doc.services = newServices;
	return dumpDoc(doc);
}

export function setImage(yamlText: string, image: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const key = firstSvcKey ?? 'app';
	ensureService(doc, key).image = image;
	return dumpDoc(doc);
}

export function setContainerName(yamlText: string, value: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (value) svc.container_name = value;
	else delete svc.container_name;
	return dumpDoc(doc);
}

export function setNetwork(yamlText: string, value: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (value && value !== 'bridge') svc.network_mode = value;
	else delete svc.network_mode;
	return dumpDoc(doc);
}

export function setCommand(yamlText: string, value: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (value) svc.command = value;
	else delete svc.command;
	return dumpDoc(doc);
}

export function setUser(yamlText: string, value: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (value) svc.user = value;
	else delete svc.user;
	return dumpDoc(doc);
}

export function setWorkingDir(yamlText: string, value: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (value) svc.working_dir = value;
	else delete svc.working_dir;
	return dumpDoc(doc);
}

export function setPrivileged(yamlText: string, value: boolean): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (value) svc.privileged = true;
	else delete svc.privileged;
	return dumpDoc(doc);
}

export function setRestart(yamlText: string, value: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	svc.restart = value || 'no';
	return dumpDoc(doc);
}

export function setMemLimitMb(yamlText: string, mb: number): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (mb > 0) {
		svc.deploy = { resources: { limits: { memory: `${mb}m` } } };
		delete svc.mem_limit;
	} else {
		delete svc.deploy;
		delete svc.mem_limit;
	}
	return dumpDoc(doc);
}

export function setIcon(yamlText: string, value: string): string {
	const { doc } = loadDoc(yamlText);
	if (value) doc['x-icon'] = value;
	else delete doc['x-icon'];
	return dumpDoc(doc);
}

export function setWebPort(yamlText: string, value: string): string {
	const { doc } = loadDoc(yamlText);
	const ext = (doc['x-powerlab'] as Record<string, unknown>) ?? {};
	if (value) ext.port_map = value;
	else delete ext.port_map;
	if (Object.keys(ext).length > 0) doc['x-powerlab'] = ext;
	else delete doc['x-powerlab'];
	return dumpDoc(doc);
}

// ─── Array entry editors: ports / volumes / devices ──────────────────────────
// Each "set" preserves the entry's original shape (string short-form
// vs object long-form). Add() emits short-form by default.

type StringOrObject = string | Record<string, unknown>;

function setPortField(
	entry: StringOrObject,
	field: 'host' | 'container',
	value: string
): StringOrObject {
	if (typeof entry === 'string') {
		const [host, container] = entry.split(':');
		const h = field === 'host' ? value : (host ?? '');
		const c = field === 'container' ? value : (container ?? '');
		return `${h}:${c}`;
	}
	const next = { ...entry } as Record<string, unknown>;
	if (field === 'host') next.published = value;
	else next.target = value;
	return next;
}

function setVolumeField(
	entry: StringOrObject,
	field: 'host' | 'container',
	value: string
): StringOrObject {
	if (typeof entry === 'string') {
		const [host, container] = entry.split(':');
		const h = field === 'host' ? value : (host ?? '');
		const c = field === 'container' ? value : (container ?? '');
		return `${h}:${c}`;
	}
	const next = { ...entry } as Record<string, unknown>;
	if (field === 'host') next.source = value;
	else next.target = value;
	return next;
}

function setDeviceField(
	entry: StringOrObject,
	field: 'host' | 'container',
	value: string
): StringOrObject {
	if (typeof entry === 'string') {
		const [host, container] = entry.split(':');
		const h = field === 'host' ? value : (host ?? '');
		const c = field === 'container' ? value : (container ?? '');
		return `${h}:${c}`;
	}
	// Devices don't have a standardised long-form in docker-compose;
	// rare. Fall back to a short-form string.
	return `${field === 'host' ? value : ''}:${field === 'container' ? value : ''}`;
}

function arrayKey(
	field: 'ports' | 'volumes' | 'devices'
): 'ports' | 'volumes' | 'devices' {
	return field;
}

function ensureArray(svc: Service, key: 'ports' | 'volumes' | 'devices'): unknown[] {
	if (!Array.isArray(svc[key])) svc[key] = [];
	return svc[key] as unknown[];
}

export function setPortAt(
	yamlText: string,
	index: number,
	field: 'host' | 'container',
	value: string
): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const arr = ensureArray(svc, 'ports');
	if (arr[index] === undefined) arr[index] = '';
	arr[index] = setPortField(arr[index] as StringOrObject, field, value);
	return dumpDoc(doc);
}

export function setVolumeAt(
	yamlText: string,
	index: number,
	field: 'host' | 'container',
	value: string
): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const arr = ensureArray(svc, 'volumes');
	if (arr[index] === undefined) arr[index] = '';
	arr[index] = setVolumeField(arr[index] as StringOrObject, field, value);
	return dumpDoc(doc);
}

export function setDeviceAt(
	yamlText: string,
	index: number,
	field: 'host' | 'container',
	value: string
): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const arr = ensureArray(svc, 'devices');
	if (arr[index] === undefined) arr[index] = '';
	arr[index] = setDeviceField(arr[index] as StringOrObject, field, value);
	return dumpDoc(doc);
}

export function addPort(yamlText: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const arr = ensureArray(svc, 'ports');
	arr.push('');
	return dumpDoc(doc);
}
export function addVolume(yamlText: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const arr = ensureArray(svc, 'volumes');
	arr.push('');
	return dumpDoc(doc);
}
export function addDevice(yamlText: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const arr = ensureArray(svc, 'devices');
	arr.push('');
	return dumpDoc(doc);
}

export function removeArrayItem(
	yamlText: string,
	field: 'ports' | 'volumes' | 'devices',
	index: number
): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = doc.services?.[firstSvcKey ?? ''];
	if (!svc) return yamlText;
	const arr = svc[arrayKey(field)];
	if (!Array.isArray(arr) || index < 0 || index >= arr.length) return yamlText;
	arr.splice(index, 1);
	if (arr.length === 0) delete svc[arrayKey(field)];
	return dumpDoc(doc);
}

// ─── env / labels (key=value lists or maps) ──────────────────────────────────

function envEntriesToYaml(entries: Array<{ key: string; value: string }>): string[] {
	return entries.filter((e) => e.key).map((e) => `${e.key}=${e.value}`);
}

export function setEnvAt(
	yamlText: string,
	index: number,
	field: 'key' | 'value',
	value: string
): string {
	const view = viewFromYaml(yamlText);
	const list = [...view.env];
	while (list.length <= index) list.push({ key: '', value: '' });
	list[index] = { ...list[index], [field]: value };
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const filtered = list.filter((e) => e.key || e.value);
	if (filtered.length > 0) svc.environment = envEntriesToYaml(filtered);
	else delete svc.environment;
	return dumpDoc(doc);
}

export function addEnv(yamlText: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (!Array.isArray(svc.environment)) {
		svc.environment = [];
	}
	(svc.environment as string[]).push('=');
	return dumpDoc(doc);
}

export function removeEnvAt(yamlText: string, index: number): string {
	const view = viewFromYaml(yamlText);
	const list = view.env.filter((_, i) => i !== index);
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (list.length === 0) delete svc.environment;
	else svc.environment = envEntriesToYaml(list);
	return dumpDoc(doc);
}

export function setLabelAt(
	yamlText: string,
	index: number,
	field: 'key' | 'value',
	value: string
): string {
	const view = viewFromYaml(yamlText);
	const list = [...view.labels];
	while (list.length <= index) list.push({ key: '', value: '' });
	list[index] = { ...list[index], [field]: value };
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	const filtered = list.filter((e) => e.key);
	if (filtered.length > 0) {
		const obj: Record<string, string> = {};
		for (const e of filtered) obj[e.key] = e.value;
		svc.labels = obj;
	} else {
		delete svc.labels;
	}
	return dumpDoc(doc);
}

export function addLabel(yamlText: string): string {
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (!svc.labels || typeof svc.labels !== 'object' || Array.isArray(svc.labels)) {
		svc.labels = {};
	}
	const labels = svc.labels as Record<string, string>;
	labels[`new.label.${Object.keys(labels).length}`] = '';
	return dumpDoc(doc);
}

export function removeLabelAt(yamlText: string, index: number): string {
	const view = viewFromYaml(yamlText);
	const list = view.labels.filter((_, i) => i !== index);
	const { doc, firstSvcKey } = loadDoc(yamlText);
	const svc = ensureService(doc, firstSvcKey ?? 'app');
	if (list.length === 0) {
		delete svc.labels;
	} else {
		const obj: Record<string, string> = {};
		for (const e of list) obj[e.key] = e.value;
		svc.labels = obj;
	}
	return dumpDoc(doc);
}
