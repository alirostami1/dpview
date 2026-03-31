---
title: Theme Comparison
author:
  name: Aros
tags:
  - demo
  - markdown
published: true
---

# Theme Comparison

This Markdown sample exists to compare the selected preview theme with the Typst sample next to it.

## Text And Links

The quick brown fox jumps over the lazy dog. Visit [Example](https://example.com) to confirm link styling.

> Blockquotes should make the accent color obvious in every preview theme.

## Lists

- First item
- Second item
- [x] Completed task
- [ ] Open task

## Table

| Theme   | Goal                     |
| ------- | ------------------------ |
| Default | Neutral baseline         |
| GitHub  | Familiar docs styling    |
| Notion  | Softer workspace styling |
| Paper   | Warm reading surface     |

## Code

Inline code like `preview-theme` should pick up the theme palette.

```go
package main

import "fmt"

func main() {
	fmt.Println("markdown sample")
}
```

## Math

Inline math like $e^{i\pi} + 1 = 0$ should render inside paragraphs.

Display math should also work:

$$
\int_{-\infty}^{\infty} e^{-x^2} \, dx = \sqrt{\pi}
$$

## Footnotes

Footnotes now render in the preview.[^intro] Repeated references should also work.[^repeat]

The same note can be referenced more than once in the same document.[^repeat]

[^intro]: This footnote demonstrates the standard Goldmark footnote syntax.

[^repeat]: This shared footnote shows backlink handling for repeated references.
