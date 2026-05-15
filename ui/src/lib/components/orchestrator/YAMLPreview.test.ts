import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import YAMLPreview from './YAMLPreview.svelte';

// YAML-first preview locks the bug class that bit the prior visual
// orchestrator form: long-form volumes / devices / port objects must
// not surface as "[object Object]" anywhere in the rendered output.
//
// Replaces the role of orchestrator.sync.test.ts +
// fork-volume-shape.test.ts under the YAML-first design.

const FORK_VOLUMES_LONG_FORM = `name: forkedapp
services:
  app:
    image: nginx:alpine
    volumes:
      - type: bind
        source: /var/lib/powerlab/apps/forkedapp/data
        target: /app/data
      - type: bind
        source: /var/lib/powerlab/apps/forkedapp/conf
        target: /etc/conf
    devices:
      - /dev/dri:/dev/dri
`;

const PORTS_LONG_FORM = `name: portsapp
services:
  web:
    image: nginx
    ports:
      - target: 80
        published: 8080
        protocol: tcp
        mode: host
`;

const JELLYFIN_LIKE = `name: jellyfin
services:
  jellyfin:
    image: lscr.io/linuxserver/jellyfin:latest
    ports:
      - "8096:8096"
      - "8920:8920"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/Sao_Paulo
    volumes:
      - /DATA/AppData/jellyfin/config:/config
      - /DATA/AppData/jellyfin/cache:/cache
      - /DATA/Media:/media
`;

const POSTGRES_LIKE = `name: postgresapp
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: example
    volumes:
      - pgdata:/var/lib/postgresql/data
  app:
    image: my-app:latest
    depends_on:
      - db
volumes:
  pgdata:
networks:
  default:
    driver: bridge
`;

describe('YAMLPreview', () => {
	it('renders empty hint when input blank', () => {
		render(YAMLPreview, { props: { yaml: '' } });
		expect(screen.getByText(/paste a compose yaml/i)).toBeTruthy();
	});

	it('renders parse error for malformed YAML', () => {
		render(YAMLPreview, { props: { yaml: ': : :' } });
		const err = screen.getByTestId('yaml-preview-parse-error');
		expect(err).toBeTruthy();
	});

	it('renders project name + service count for a simple app', () => {
		render(YAMLPreview, { props: { yaml: JELLYFIN_LIKE } });
		expect(screen.getByTestId('yaml-preview-project-name').textContent).toBe('jellyfin');
		expect(screen.getByTestId('yaml-preview-service-count').textContent).toMatch(/1 service\b/);
	});

	it('renders ports/volumes/env counts for jellyfin-like YAML', () => {
		render(YAMLPreview, { props: { yaml: JELLYFIN_LIKE } });
		expect(screen.getByTestId('yaml-preview-service-ports').textContent).toMatch(/8096:8096/);
		expect(screen.getByTestId('yaml-preview-service-volumes').textContent).toMatch(/3 volumes/);
		expect(screen.getByTestId('yaml-preview-service-env').textContent).toMatch(/3 env/);
	});

	// Critical regression case — long-form volume objects must NOT
	// render as "[object Object]" anywhere.
	it('long-form volume objects do not surface as [object Object]', () => {
		const { container } = render(YAMLPreview, { props: { yaml: FORK_VOLUMES_LONG_FORM } });
		const text = container.textContent ?? '';
		expect(text).not.toContain('[object Object]');
		// Should still count the 2 long-form volumes.
		expect(screen.getByTestId('yaml-preview-service-volumes').textContent).toMatch(/2 volumes/);
	});

	it('long-form port objects format as published:target without [object Object]', () => {
		const { container } = render(YAMLPreview, { props: { yaml: PORTS_LONG_FORM } });
		expect(container.textContent).not.toContain('[object Object]');
		expect(screen.getByTestId('yaml-preview-service-ports').textContent).toMatch(/8080:80/);
	});

	it('multi-service compose lists each service + top-level volumes', () => {
		render(YAMLPreview, { props: { yaml: POSTGRES_LIKE } });
		expect(screen.getByTestId('yaml-preview-service-count').textContent).toMatch(/2 services\b/);
		// Top-level named volumes panel.
		const named = screen.getByTestId('yaml-preview-top-volumes');
		expect(named.textContent).toMatch(/pgdata/);
		// Top-level networks panel.
		const nets = screen.getByTestId('yaml-preview-top-networks');
		expect(nets.textContent).toMatch(/default/);
	});

	it('parametric: no [object Object] across diverse real-world YAMLs', () => {
		const corpus = [JELLYFIN_LIKE, POSTGRES_LIKE, FORK_VOLUMES_LONG_FORM, PORTS_LONG_FORM];
		for (const yamlText of corpus) {
			const { container, unmount } = render(YAMLPreview, { props: { yaml: yamlText } });
			expect(container.textContent, `case: ${yamlText.slice(0, 40)}…`).not.toContain('[object Object]');
			unmount();
		}
	});
});
