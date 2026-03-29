# Markdown Theming Contract

Markdown themes in DPview are interchangeable CSS files. Each theme must style the same root class and the same descendant element set.

## Root Class

Rendered Markdown is wrapped as:

```html
<article class="markdown-theme">...</article>
```

Theme stylesheets must target `.markdown-theme` as the only public root selector.

## Theme File Convention

Markdown theme files live at:

```text
app/cmd/web/themes/markdown/<theme>.css
```

The `<theme>` filename must match the frontend theme id stored in settings.

## Supported Selector Contract

Theme files may style these shared selectors under `.markdown-theme`:

- `.markdown-theme`
- `.markdown-theme h1`
- `.markdown-theme h2`
- `.markdown-theme h3`
- `.markdown-theme h4`
- `.markdown-theme h5`
- `.markdown-theme h6`
- `.markdown-theme p`
- `.markdown-theme a`
- `.markdown-theme ul`
- `.markdown-theme ol`
- `.markdown-theme li`
- `.markdown-theme input[type="checkbox"]`
- `.markdown-theme code`
- `.markdown-theme pre`
- `.markdown-theme pre code`
- `.markdown-theme blockquote`
- `.markdown-theme table`
- `.markdown-theme thead`
- `.markdown-theme tbody`
- `.markdown-theme tr`
- `.markdown-theme th`
- `.markdown-theme td`
- `.markdown-theme hr`
- `.markdown-theme img`

Themes should avoid relying on theme-specific class names or extra wrapper classes.

## Light and Dark Support

Each Markdown theme should support both resolved app theme modes:

- `body[data-theme="light"] .markdown-theme ...`
- `body[data-theme="dark"] .markdown-theme ...`

The global app theme decides whether the light or dark branch is active. Markdown theme selection only swaps the active stylesheet file.
