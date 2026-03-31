import katex from "katex";
import renderMathInElement from "katex/contrib/auto-render";

export function renderMarkdownMath(container: Element | null): void {
  if (!container) {
    return;
  }
  for (const node of container.querySelectorAll<HTMLElement>(
    ".markdown-math-block"
  )) {
    katex.render(node.getAttribute("data-latex") || "", node, {
      displayMode: true,
      throwOnError: false,
    });
  }
  renderMathInElement(container, {
    delimiters: [
      { left: "$$", right: "$$", display: true },
      { left: "$", right: "$", display: false },
      { left: "\\(", right: "\\)", display: false },
      { left: "\\[", right: "\\]", display: true },
    ],
    throwOnError: false,
  });
}
