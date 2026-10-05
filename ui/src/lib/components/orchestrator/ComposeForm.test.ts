import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import { load } from 'js-yaml';
import ComposeForm from './ComposeForm.svelte';

// The form fires a debounced port-availability check; keep it offline.
vi.mock('$lib/api/apps', () => ({
	checkPorts: vi.fn().mockResolvedValue({ data: {}, suggestions: {} })
}));

// #543: the compose project `name:` is the app's identity (same name =
// same app, updated in place). It must be a first-class form input that
// round-trips with the YAML, not something only reachable in the editor.

const YAML = `name: my-app
services:
  web:
    image: nginx:alpine
`;

describe('ComposeForm — project name (#543)', () => {
	it('renders the compose project name from the YAML', () => {
		render(ComposeForm, { props: { yaml: YAML, onChange: vi.fn() } });
		const input = screen.getByTestId('project-name-input') as HTMLInputElement;
		expect(input.value).toBe('my-app');
		expect(screen.getByLabelText('App Name (Project)')).toBe(input);
		expect(screen.getByText(/reusing an existing name updates that app/)).toBeTruthy();
	});

	it('editing the input writes the top-level name back to the YAML', async () => {
		const onChange = vi.fn();
		render(ComposeForm, { props: { yaml: YAML, onChange } });
		const input = screen.getByTestId('project-name-input');
		await fireEvent.input(input, { target: { value: 'my-other-app' } });
		expect(onChange).toHaveBeenCalledTimes(1);
		const doc = load(onChange.mock.calls[0][0]) as {
			name: string;
			services: Record<string, { image: string }>;
		};
		expect(doc.name).toBe('my-other-app');
		// Service key and image untouched by a project rename.
		expect(Object.keys(doc.services)).toEqual(['web']);
		expect(doc.services.web.image).toBe('nginx:alpine');
	});

	it('re-renders the new name when the parent feeds the YAML back', async () => {
		const onChange = vi.fn();
		const { rerender } = render(ComposeForm, { props: { yaml: YAML, onChange } });
		await fireEvent.input(screen.getByTestId('project-name-input'), {
			target: { value: 'renamed' }
		});
		await rerender({ yaml: onChange.mock.calls[0][0], onChange });
		expect((screen.getByTestId('project-name-input') as HTMLInputElement).value).toBe('renamed');
	});
});
