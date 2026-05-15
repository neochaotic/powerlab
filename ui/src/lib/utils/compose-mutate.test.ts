import { describe, it, expect } from 'vitest';
import yaml from 'js-yaml';
import {
	viewFromYaml,
	setProjectName,
	setServiceName,
	setImage,
	setNetwork,
	setCommand,
	setMemLimitMb,
	setIcon,
	setWebPort,
	setPortAt,
	setVolumeAt,
	addPort,
	addVolume,
	addEnv,
	setEnvAt,
	removeArrayItem,
	removeEnvAt,
	addLabel,
	setLabelAt,
	setPrivileged,
	setRestart
} from './compose-mutate';

// Round-trip helper: parse the output to assert structure.
const parse = (s: string) => yaml.load(s) as Record<string, unknown>;

const SHORT_FORM = `name: shortapp
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    volumes:
      - /data:/var/lib/data
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
`;

const LONG_FORM = `name: longapp
services:
  app:
    image: nginx:alpine
    ports:
      - target: 80
        published: 8080
        protocol: tcp
        mode: host
    volumes:
      - type: bind
        source: /var/lib/powerlab/apps/longapp/data
        target: /app/data
`;

const POSTGRES_LIKE = `name: postgresapp
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: example
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`;

describe('viewFromYaml', () => {
	it('returns empty view for blank input', () => {
		const v = viewFromYaml('');
		expect(v.serviceName).toBe('');
		expect(v.image).toBe('');
		expect(v.ports).toEqual([]);
	});

	it('reads scalar fields from short-form compose', () => {
		const v = viewFromYaml(SHORT_FORM);
		expect(v.projectName).toBe('shortapp');
		expect(v.serviceName).toBe('web');
		expect(v.image).toBe('nginx:latest');
	});

	it('parses short-form ports + volumes into host/container view', () => {
		const v = viewFromYaml(SHORT_FORM);
		expect(v.ports).toEqual([
			expect.objectContaining({ host: '8080', container: '80', raw: '8080:80' })
		]);
		expect(v.volumes).toEqual([
			expect.objectContaining({ host: '/data', container: '/var/lib/data' })
		]);
	});

	it('parses long-form port object into host/container view, raw preserves shape', () => {
		const v = viewFromYaml(LONG_FORM);
		expect(v.ports.length).toBe(1);
		expect(v.ports[0].host).toBe('8080');
		expect(v.ports[0].container).toBe('80');
		expect(v.ports[0].raw).toEqual(
			expect.objectContaining({ published: 8080, target: 80, protocol: 'tcp' })
		);
	});

	it('parses long-form volume object into host/container view, raw preserves shape', () => {
		const v = viewFromYaml(LONG_FORM);
		expect(v.volumes[0].host).toBe('/var/lib/powerlab/apps/longapp/data');
		expect(v.volumes[0].container).toBe('/app/data');
		expect(v.volumes[0].raw).toEqual(
			expect.objectContaining({ type: 'bind', source: expect.any(String) })
		);
	});

	it('parses env from array (KEY=VALUE) form', () => {
		const v = viewFromYaml(SHORT_FORM);
		expect(v.env).toContainEqual({ key: 'DB_HOST', value: 'postgres' });
		expect(v.env).toContainEqual({ key: 'DB_PORT', value: '5432' });
	});

	it('parses env from object form', () => {
		const v = viewFromYaml(POSTGRES_LIKE);
		expect(v.env).toContainEqual({ key: 'POSTGRES_PASSWORD', value: 'example' });
	});
});

describe('scalar setters', () => {
	it('setImage updates the first service image', () => {
		const out = setImage(SHORT_FORM, 'nginx:1.27');
		const doc = parse(out);
		const services = doc.services as Record<string, Record<string, unknown>>;
		expect(services.web.image).toBe('nginx:1.27');
	});

	it('setProjectName updates top-level name', () => {
		const out = setProjectName(SHORT_FORM, 'renamed');
		expect(parse(out).name).toBe('renamed');
	});

	it('setServiceName renames the first service key', () => {
		const out = setServiceName(SHORT_FORM, 'newweb');
		const doc = parse(out);
		const services = doc.services as Record<string, unknown>;
		expect(Object.keys(services)).toEqual(['newweb']);
		expect((services.newweb as Record<string, unknown>).image).toBe('nginx:latest');
	});

	it('setNetwork stores in network_mode, omits when default bridge', () => {
		const out = setNetwork(SHORT_FORM, 'host');
		expect(parse(out).services).toMatchObject({ web: { network_mode: 'host' } });

		const cleared = setNetwork(SHORT_FORM, 'bridge');
		const svc = (parse(cleared).services as Record<string, Record<string, unknown>>).web;
		expect(svc.network_mode).toBeUndefined();
	});

	it('setCommand omits when empty', () => {
		const out = setCommand(SHORT_FORM, '');
		const svc = (parse(out).services as Record<string, Record<string, unknown>>).web;
		expect(svc.command).toBeUndefined();
	});

	it('setMemLimitMb writes deploy.resources.limits.memory', () => {
		const out = setMemLimitMb(SHORT_FORM, 1024);
		const svc = (parse(out).services as Record<string, Record<string, unknown>>).web;
		const deploy = svc.deploy as { resources?: { limits?: { memory?: string } } };
		expect(deploy.resources?.limits?.memory).toBe('1024m');
	});

	it('setIcon writes top-level x-icon', () => {
		const out = setIcon(SHORT_FORM, 'https://example.com/x.png');
		expect(parse(out)['x-icon']).toBe('https://example.com/x.png');
	});

	it('setWebPort writes x-powerlab.port_map', () => {
		const out = setWebPort(SHORT_FORM, '8080');
		expect(((parse(out)['x-powerlab'] as Record<string, unknown>) || {}).port_map).toBe('8080');
	});

	it('setPrivileged true adds; false removes', () => {
		const on = setPrivileged(SHORT_FORM, true);
		const onSvc = (parse(on).services as Record<string, Record<string, unknown>>).web;
		expect(onSvc.privileged).toBe(true);
		const off = setPrivileged(on, false);
		const offSvc = (parse(off).services as Record<string, Record<string, unknown>>).web;
		expect(offSvc.privileged).toBeUndefined();
	});

	it('setRestart accepts a value', () => {
		const out = setRestart(SHORT_FORM, 'unless-stopped');
		const svc = (parse(out).services as Record<string, Record<string, unknown>>).web;
		expect(svc.restart).toBe('unless-stopped');
	});
});

// CRITICAL: long-form preservation. This is the bug class #332 we
// MUST defeat — editing one field on a long-form entry must keep
// the entry long-form, not collapse it to "[object Object]" or lose
// the structure.
describe('array mutations preserve entry shape', () => {
	it('setPortAt on a short-form entry keeps it short-form', () => {
		const out = setPortAt(SHORT_FORM, 0, 'host', '9090');
		const ports = (parse(out).services as Record<string, Record<string, unknown>>).web.ports;
		expect(ports).toEqual(['9090:80']);
	});

	it('setPortAt on a long-form entry keeps it long-form + preserves siblings', () => {
		const out = setPortAt(LONG_FORM, 0, 'host', '9090');
		const ports = (parse(out).services as Record<string, Record<string, unknown>>).app.ports;
		expect(Array.isArray(ports)).toBe(true);
		expect((ports as unknown[])[0]).toEqual(
			expect.objectContaining({
				published: '9090',
				target: 80,
				protocol: 'tcp',
				mode: 'host'
			})
		);
	});

	it('setVolumeAt on a long-form entry preserves type + source/target shape', () => {
		const out = setVolumeAt(LONG_FORM, 0, 'host', '/new/path');
		const vols = (parse(out).services as Record<string, Record<string, unknown>>).app.volumes;
		expect((vols as unknown[])[0]).toEqual(
			expect.objectContaining({ type: 'bind', source: '/new/path', target: '/app/data' })
		);
	});

	it('setVolumeAt on a short-form entry keeps it short-form', () => {
		const out = setVolumeAt(SHORT_FORM, 0, 'container', '/new/data');
		const vols = (parse(out).services as Record<string, Record<string, unknown>>).web.volumes;
		expect(vols).toEqual(['/data:/new/data']);
	});

	it('round-trip of a long-form YAML through view + setPortAt does NOT produce [object Object]', () => {
		const out = setPortAt(LONG_FORM, 0, 'host', '9090');
		expect(out).not.toContain('[object Object]');
		expect(out).not.toContain('undefined');
	});

	it('addPort appends an empty short-form slot', () => {
		const out = addPort(SHORT_FORM);
		const ports = (parse(out).services as Record<string, Record<string, unknown>>).web.ports;
		expect((ports as unknown[]).length).toBe(2);
	});

	it('addVolume appends an empty short-form slot', () => {
		const out = addVolume(SHORT_FORM);
		const vols = (parse(out).services as Record<string, Record<string, unknown>>).web.volumes;
		expect((vols as unknown[]).length).toBe(2);
	});

	it('removeArrayItem on ports trims the array', () => {
		const augmented = addPort(SHORT_FORM); // 2 ports
		const trimmed = removeArrayItem(augmented, 'ports', 1);
		const ports = (parse(trimmed).services as Record<string, Record<string, unknown>>).web.ports;
		expect((ports as unknown[]).length).toBe(1);
	});

	it('removeArrayItem clearing last entry deletes the field', () => {
		const trimmed = removeArrayItem(SHORT_FORM, 'ports', 0);
		const svc = (parse(trimmed).services as Record<string, Record<string, unknown>>).web;
		expect(svc.ports).toBeUndefined();
	});
});

describe('env / labels editors', () => {
	it('setEnvAt updates key on a string-list env', () => {
		const out = setEnvAt(SHORT_FORM, 0, 'value', 'newhost');
		const env = (parse(out).services as Record<string, Record<string, unknown>>).web
			.environment as string[];
		expect(env[0]).toBe('DB_HOST=newhost');
		expect(env[1]).toBe('DB_PORT=5432');
	});

	it('addEnv appends + setEnvAt then writes', () => {
		const a = addEnv(SHORT_FORM);
		const b = setEnvAt(a, 2, 'key', 'NEW');
		const c = setEnvAt(b, 2, 'value', 'value');
		const env = (parse(c).services as Record<string, Record<string, unknown>>).web
			.environment as string[];
		expect(env).toContain('NEW=value');
	});

	it('removeEnvAt drops the entry', () => {
		const out = removeEnvAt(SHORT_FORM, 0);
		const env = (parse(out).services as Record<string, Record<string, unknown>>).web
			.environment as string[];
		expect(env).not.toContain(expect.stringContaining('DB_HOST'));
	});

	it('setLabelAt + addLabel write to labels map', () => {
		const a = addLabel(SHORT_FORM);
		const b = setLabelAt(a, 0, 'key', 'com.test');
		const c = setLabelAt(b, 0, 'value', 'on');
		const labels = (parse(c).services as Record<string, Record<string, unknown>>).web.labels;
		expect(labels).toMatchObject({ 'com.test': 'on' });
	});
});

describe('parametric: no [object Object] across diverse YAMLs after edits', () => {
	const corpus = [SHORT_FORM, LONG_FORM, POSTGRES_LIKE];
	for (const seed of corpus) {
		it(`edits across ${seed.slice(0, 30).replace(/\n/g, ' ')}…`, () => {
			let s = seed;
			s = setImage(s, 'nginx:1.27');
			s = setProjectName(s, 'edited');
			s = setMemLimitMb(s, 1024);
			s = setIcon(s, 'http://test/icon.png');
			s = setWebPort(s, '8080');
			s = addPort(s);
			s = setPortAt(s, 1, 'host', '9999');
			s = setPortAt(s, 1, 'container', '999');
			s = addVolume(s);
			s = setVolumeAt(s, 1, 'host', '/data');
			s = setVolumeAt(s, 1, 'container', '/c');
			s = addEnv(s);
			s = setEnvAt(s, 0, 'key', 'EDITED');
			expect(s).not.toContain('[object Object]');
			expect(s).not.toContain('undefined');
			// Must parse back to a valid compose doc.
			const re = parse(s);
			expect(re).toBeDefined();
		});
	}
});
