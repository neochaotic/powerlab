/**
 * Regression test for issue #332 — Custom App fork volumes form
 * showing "[object Object]".
 *
 * Compose YAML supports two volume forms:
 *   short  →  "/host/path:/container/path"   (string)
 *   long   →  {type: bind, source: /host/path, target: /container/path}
 *
 * /apps/new's syncYamlToForm() did `String(v).split(':')` against
 * each entry. For long-form objects this produced "[object Object]"
 * which got crammed into the host field; the form rendered that
 * literal in the volumes UI. Same trap on `service.devices`.
 *
 * Test mounts the page with a fork-style YAML containing a long-form
 * volume + a long-form device and asserts the rendered form fields
 * show the correct paths, not "[object Object]".
 */

import { describe, it, expect } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import { tick } from 'svelte';
import Page from './+page.svelte';

const YAML_WITH_LONG_FORM_VOLUMES = `name: forkedapp
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

describe('Orchestrator — fork with long-form volumes (issue #332)', () => {
	it('parses long-form volume objects into host:container without "[object Object]"', async () => {
		const { container } = render(Page);
		await tick();

		const textarea = container.querySelector('textarea')!;
		await fireEvent.input(textarea, { target: { value: YAML_WITH_LONG_FORM_VOLUMES } });
		await tick();
		await tick();

		const text = container.textContent ?? '';
		expect(text).not.toContain('[object Object]');
	});

	it('extracts source + target from long-form volume into the volumes form', async () => {
		const { container } = render(Page);
		await tick();
		const textarea = container.querySelector('textarea')!;
		await fireEvent.input(textarea, { target: { value: YAML_WITH_LONG_FORM_VOLUMES } });
		await tick();
		await tick();

		// The volumes form renders inputs with values bound to host
		// and container. Find input values matching either field.
		const inputs = Array.from(container.querySelectorAll('input')) as HTMLInputElement[];
		const values = inputs.map((i) => i.value).filter(Boolean);

		expect(values.some((v) => v.includes('/var/lib/powerlab/apps/forkedapp/data'))).toBe(true);
		expect(values.some((v) => v.includes('/app/data'))).toBe(true);
	});
});
