/**
 * Regression tests for the Orchestrator bi-directional sync.
 *
 * Two classes of bugs that Antigravity introduced, caught by these tests:
 *
 * Bug 1 — $effect + untrack (YAML → Form sync broken):
 *   WRONG:  $effect(() => { untrack(() => { yamlText; syncYamlToForm(); }); });
 *   RIGHT:  $effect(() => { yamlText; untrack(() => syncYamlToForm()); });
 *   Root cause: wrapping the YAML read inside untrack() means $effect never
 *   registers yamlText as a dependency — the effect only runs on mount, never
 *   again. Editing the YAML editor produces no form updates.
 *
 * Bug 2 — onchange prop casing (Form → YAML sync broken):
 *   WRONG:  <ComposeForm onChange={handleFormChange}>
 *   RIGHT:  <ComposeForm onchange={handleFormChange}>
 *   Root cause: Svelte 5 component props are case-sensitive. `onChange` is a
 *   different (undefined) prop — the callback is silently discarded.
 *   Editing any form field produces no YAML update.
 */
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import { fireEvent } from '@testing-library/svelte';
import { tick } from 'svelte';
import Page from '../routes/apps/new/+page.svelte';

const YAML_REDIS = `version: '3.8'\nservices:\n  myapp:\n    image: redis:7\n    restart: always\n`;

describe('Orchestrator — YAML → Form sync', () => {
	it('initial YAML populates service name and image on mount', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		const nameInput = container.querySelector('#service-name') as HTMLInputElement;
		const imageInput = container.querySelector('#docker-image') as HTMLInputElement;

		expect(nameInput?.value).toBe('web');
		expect(imageInput?.value).toBe('nginx:latest');
	});

	/**
	 * This test catches Bug 1: if $effect wraps yamlText access inside
	 * untrack(), this test fails because the form never updates after
	 * the textarea changes.
	 */
	it('editing the YAML textarea updates the image form field', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		const textarea = container.querySelector('textarea')!;
		fireEvent.input(textarea, { target: { value: YAML_REDIS } });
		await tick();
		await tick();

		const imageInput = container.querySelector('#docker-image') as HTMLInputElement;
		expect(imageInput?.value).toBe('redis:7');
	});

	it('editing the YAML textarea updates the service name form field', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		const textarea = container.querySelector('textarea')!;
		fireEvent.input(textarea, { target: { value: YAML_REDIS } });
		await tick();
		await tick();

		const nameInput = container.querySelector('#service-name') as HTMLInputElement;
		expect(nameInput?.value).toBe('myapp');
	});

	it('invalid YAML in textarea does not crash the form', async () => {
		const { container } = render(Page);
		await tick();

		const textarea = container.querySelector('textarea')!;
		fireEvent.input(textarea, { target: { value: '{ not: valid: yaml: [' } });
		await tick();

		// Form must still be present — no crash, no blank screen
		expect(container.querySelector('#docker-image')).toBeTruthy();
	});
});

describe('Orchestrator — Form → YAML sync', () => {
	/**
	 * This test catches Bug 2: if the prop is written as `onChange` instead of
	 * `onchange`, handleFormChange is never called, syncFormToYaml never runs,
	 * and the textarea stays unchanged after editing a form field.
	 */
	it('editing the image input updates the YAML textarea', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		const imageInput = container.querySelector('#docker-image') as HTMLInputElement;
		fireEvent.input(imageInput, { target: { value: 'redis:7' } });
		await tick();
		await tick();

		const textarea = container.querySelector('textarea')!;
		expect(textarea.value).toContain('redis:7');
	});

	it('editing the service name input updates the YAML textarea', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		const nameInput = container.querySelector('#service-name') as HTMLInputElement;
		fireEvent.input(nameInput, { target: { value: 'myservice' } });
		await tick();
		await tick();

		const textarea = container.querySelector('textarea')!;
		expect(textarea.value).toContain('myservice');
	});

	it('YAML textarea always contains valid YAML after form edits', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		const imageInput = container.querySelector('#docker-image') as HTMLInputElement;
		fireEvent.input(imageInput, { target: { value: 'postgres:16' } });
		await tick();
		await tick();

		const textarea = container.querySelector('textarea')!;
		// Must contain the YAML version header and services key
		expect(textarea.value).toContain('version:');
		expect(textarea.value).toContain('services:');
		expect(textarea.value).toContain('image: postgres:16');
	});
});

describe('Orchestrator — round-trip sync', () => {
	it('YAML → Form → YAML preserves the image value', async () => {
		const { container } = render(Page);
		await tick();
		await tick();

		// Step 1: push new YAML into textarea
		const textarea = container.querySelector('textarea')!;
		fireEvent.input(textarea, { target: { value: YAML_REDIS } });
		await tick();
		await tick();

		// Step 2: confirm form reflects new value
		const imageInput = container.querySelector('#docker-image') as HTMLInputElement;
		expect(imageInput?.value).toBe('redis:7');

		// Step 3: edit the form back to nginx
		fireEvent.input(imageInput, { target: { value: 'nginx:alpine' } });
		await tick();
		await tick();

		// Step 4: confirm YAML reflects the form edit
		expect(textarea.value).toContain('nginx:alpine');
	});
});
