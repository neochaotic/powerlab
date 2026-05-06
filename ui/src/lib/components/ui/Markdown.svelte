<script lang="ts">
    import { marked } from 'marked';
    import DOMPurify from 'dompurify';
    import { cn } from '$lib/utils';

    interface Props {
        content: string;
        class?: string;
    }

    let { content, class: className }: Props = $props();

    const html = $derived.by(() => {
        if (!content) return '';
        const rawHtml = marked.parse(content) as string;
        return DOMPurify.sanitize(rawHtml);
    });
</script>

<div class={cn("prose prose-invert prose-emerald max-w-none", className)}>
    {@html html}
</div>

<style>
    :global(.prose pre) {
        background-color: rgba(255, 255, 255, 0.05) !important;
        border: 1px solid rgba(255, 255, 255, 0.1);
        border-radius: 1rem;
    }
    :global(.prose code) {
        color: var(--color-emerald-400);
        background-color: rgba(16, 185, 129, 0.1);
        padding: 0.2em 0.4em;
        border-radius: 0.4em;
        font-weight: 500;
    }
    :global(.prose code::before), :global(.prose code::after) {
        content: none !important;
    }
</style>
