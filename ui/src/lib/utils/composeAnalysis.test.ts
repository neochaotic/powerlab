import { describe, it, expect } from 'vitest';
import {
	analyzeCompose,
	analyzeComposeYaml,
	parseShortPortSyntax,
	publishedHostPorts,
	resolveInterpolation,
	MAX_RANGE_EXPANSION,
	WARNING_CAP_ADD,
	WARNING_HOST_NETWORK,
	WARNING_KERNEL_DEVICES,
	WARNING_PRIVILEGED
} from './composeAnalysis';

describe('parseShortPortSyntax', () => {
	it.each<[string, number[]]>([
		['80', []], // container-only → ephemeral host port, cannot conflict
		['8080:80', [8080]],
		['127.0.0.1:8080:80', [8080]], // regression: used to read as 127
		['[::1]:8080:80', [8080]],
		['8080:80/udp', [8080]],
		['127.0.0.1:5353:53/udp', [5353]],
		['8000-8002:8000-8002', [8000, 8001, 8002]],
		['127.0.0.1:8000-8001:80', [8000, 8001]],
		['127.0.0.1::80', []], // IP with ephemeral host port
		['${PORT:-8080}:80', [8080]],
		['${PORT-9090}:80', [9090]],
		['${PORT}:80', []],
		['$PORT:80', []],
		['', []],
		['not-a-port', []],
		['70000:80', []],
		['8002-8000:80', []]
	])('%s → %j', (entry, expected) => {
		expect(parseShortPortSyntax(entry)).toEqual(expected);
	});

	it('skips ranges wider than the expansion cap', () => {
		expect(parseShortPortSyntax(`1-${MAX_RANGE_EXPANSION + 1}:1-${MAX_RANGE_EXPANSION + 1}`)).toEqual([]);
	});
});

describe('publishedHostPorts (long syntax)', () => {
	it('reads numeric published', () => {
		expect(publishedHostPorts({ target: 80, published: 8080 })).toEqual([8080]);
	});
	it('reads string published', () => {
		expect(publishedHostPorts({ target: 80, published: '8080' })).toEqual([8080]);
	});
	it('reads string range published', () => {
		expect(publishedHostPorts({ target: 80, published: '9000-9001' })).toEqual([9000, 9001]);
	});
	it('reads interpolated published', () => {
		expect(publishedHostPorts({ target: 80, published: '${WEB_PORT:-8081}' })).toEqual([8081]);
	});
	it('skips missing published', () => {
		expect(publishedHostPorts({ target: 80 })).toEqual([]);
	});
	it('skips bare numbers and junk', () => {
		expect(publishedHostPorts(80)).toEqual([]);
		expect(publishedHostPorts(null)).toEqual([]);
		expect(publishedHostPorts(['8080:80'])).toEqual([]);
	});
});

describe('resolveInterpolation', () => {
	it('substitutes defaults and rejects unresolved vars', () => {
		expect(resolveInterpolation('${A:-1}:${B-2}')).toBe('1:2');
		expect(resolveInterpolation('${A}')).toBeNull();
		expect(resolveInterpolation('plain')).toBe('plain');
	});
});

describe('analyzeCompose', () => {
	it('collects deduped ports across services and all warnings', () => {
		const result = analyzeComposeYaml(`
services:
  web:
    image: nginx
    network_mode: host
    privileged: true
    cap_add: [NET_ADMIN]
    volumes:
      - /dev/dri:/dev/dri
    ports:
      - "127.0.0.1:8080:80"
      - 80
      - target: 443
        published: "8443"
  db:
    image: postgres
    volumes:
      - type: bind
        source: /proc/cpuinfo
        target: /cpuinfo
    ports:
      - "8080:81"
      - "\${DB_PORT:-5432}:5432"
`);
		expect(result.requestedPorts).toEqual([8080, 8443, 5432]);
		expect(result.compatibilityWarnings).toEqual([
			WARNING_HOST_NETWORK,
			WARNING_PRIVILEGED,
			WARNING_CAP_ADD,
			WARNING_KERNEL_DEVICES,
			WARNING_KERNEL_DEVICES
		]);
	});

	it('ignores harmless capabilities and volumes', () => {
		const result = analyzeCompose({
			services: { a: { cap_add: ['CHOWN'], volumes: ['/data:/data', { source: 'named' }] } }
		});
		expect(result.compatibilityWarnings).toEqual([]);
	});

	it('tolerates missing or malformed documents', () => {
		expect(analyzeCompose(null)).toEqual({ requestedPorts: [], compatibilityWarnings: [] });
		expect(analyzeCompose({ services: 'nope' })).toEqual({ requestedPorts: [], compatibilityWarnings: [] });
		expect(analyzeCompose({ services: { a: null, b: { ports: 'x' } } })).toEqual({
			requestedPorts: [],
			compatibilityWarnings: []
		});
	});

	it('throws on invalid YAML (caller surfaces it)', () => {
		expect(() => analyzeComposeYaml('services: [unclosed')).toThrow();
	});
});
