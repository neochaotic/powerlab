// Mermaid.js initialiser for the PowerLab docs site.
//
// Material's navigation.instant feature swaps page content via XHR
// without a full reload, which means a one-shot mermaid.initialize()
// call only affects the page that loaded the script. This handler
// re-runs on every Material navigation event so diagrams render on
// every architecture page, not just the first one the user landed on.
//
// Reference:
//   https://squidfunk.github.io/mkdocs-material/reference/diagrams/

document$.subscribe(function () {
  const isDark =
    document.body.dataset.mdColorScheme === 'slate' ||
    window.matchMedia('(prefers-color-scheme: dark)').matches;

  mermaid.initialize({
    startOnLoad: false,
    theme: isDark ? 'dark' : 'default',
    securityLevel: 'loose',
    flowchart: { useMaxWidth: true, htmlLabels: true },
    sequence: { useMaxWidth: true },
  });

  mermaid.run({
    querySelector: '.mermaid',
  });
});
