import type { CurrentData, Preview, SeekData, Settings } from "./types";

/** Active scroller used for preview seeking. */
export type ScrollContainer = HTMLElement;

const SEEK_VIEWPORT_ANCHOR = 0.5;

/** Candidate preview element used for Markdown source-line seeking. */
interface MarkdownSeekCandidate {
  /** DOM node carrying source line metadata. */
  node: HTMLElement;
  /** First source line covered by the node. */
  start: number;
  /** Last source line covered by the node. */
  end: number;
  /** DOM depth used as a specificity tie-breaker. */
  depth: number;
}

type TypstSeekAnchor = NonNullable<Preview["typst_seek_anchors"]>[number];

/**
 * Applies editor seek state to the current preview when possible.
 *
 * @param scrollContainer Active scroll container for the current file view.
 * @param previewEl Root preview element.
 * @param current Current file/preview snapshot.
 * @param seek Current seek position snapshot.
 * @param settings Active settings snapshot.
 */
export function applyPreviewSeek(
  scrollContainer: ScrollContainer,
  previewEl: HTMLElement,
  current: CurrentData | null,
  seek: SeekData | null,
  settings: Settings
): void {
  const file = current?.file;
  const preview = current?.preview;
  if (
    !settings.seek_enabled ||
    !seek ||
    !file ||
    seek.path !== file.path ||
    preview?.status !== "ready"
  ) {
    return;
  }

  if (file.kind === "markdown") {
    applyMarkdownSeek(scrollContainer, previewEl, seek);
    return;
  }
  if (file.kind === "typst") {
    applyTypstSeek(scrollContainer, previewEl, seek, preview);
  }
}

/**
 * Aligns a Markdown preview to the best matching source line block.
 *
 * @param scrollContainer Active scroll container for the preview.
 * @param previewEl Root preview element.
 * @param seek Current seek position snapshot.
 */
function applyMarkdownSeek(
  scrollContainer: ScrollContainer,
  previewEl: HTMLElement,
  seek: SeekData
): void {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  if (!focusLine) {
    return;
  }
  const candidates = collectMarkdownSeekCandidates(previewEl);
  if (!candidates.length) {
    return;
  }

  const containing = candidates.filter(
    (candidate) => focusLine >= candidate.start && focusLine <= candidate.end
  );

  if (containing.length) {
    // Prefer the narrowest matching block so large containers do not win over the
    // specific paragraph/list item/code block the editor is currently focused on.
    const target = containing.reduce<MarkdownSeekCandidate | null>(
      (best, candidate) => {
        if (!best) {
          return candidate;
        }
        const bestSpan = best.end - best.start;
        const candidateSpan = candidate.end - candidate.start;
        if (candidateSpan !== bestSpan) {
          return candidateSpan < bestSpan ? candidate : best;
        }
        return candidate.depth > best.depth ? candidate : best;
      },
      null
    );
    scrollPreviewToLine(scrollContainer, target, focusLine);
    return;
  }

  const before = candidates
    .filter((candidate) => candidate.end < focusLine)
    .reduce<MarkdownSeekCandidate | null>(
      (best, candidate) =>
        !best || candidate.end > best.end ? candidate : best,
      null
    );
  const after = candidates
    .filter((candidate) => candidate.start > focusLine)
    .reduce<MarkdownSeekCandidate | null>(
      (best, candidate) =>
        !best || candidate.start < best.start ? candidate : best,
      null
    );

  if (before && after) {
    scrollPreviewBetweenCandidates(scrollContainer, before, after, focusLine);
    return;
  }

  scrollPreviewToLine(scrollContainer, before || after, focusLine);
}

/**
 * Aligns a Typst preview proportionally using source line progress.
 *
 * @param scrollContainer Active scroll container for the preview.
 * @param seek Current seek position snapshot.
 * @param preview Current preview data.
 */
function applyTypstSeek(
  scrollContainer: ScrollContainer,
  previewEl: HTMLElement,
  seek: SeekData,
  preview: Preview
): void {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  if (!focusLine) {
    return;
  }
  const anchors = preview.typst_seek_anchors || [];
  if (anchors.length) {
    applyTypstAnchorSeek(scrollContainer, previewEl, focusLine, anchors);
    return;
  }
  const totalLines = Number(preview.source_line_count || 0);
  if (!totalLines) {
    return;
  }
  const scrollRange =
    getScrollContainerHeight(scrollContainer) -
    getScrollContainerViewportHeight(scrollContainer);
  if (scrollRange <= 0) {
    return;
  }
  const progress =
    totalLines <= 1
      ? 0
      : Math.max(0, Math.min(1, (focusLine - 1) / (totalLines - 1)));
  const targetTop =
    progress * getScrollContainerHeight(scrollContainer) -
    getScrollContainerViewportHeight(scrollContainer) * SEEK_VIEWPORT_ANCHOR;
  setScrollContainerTop(
    scrollContainer,
    Math.max(0, Math.min(scrollRange, targetTop))
  );
}

function applyTypstAnchorSeek(
  scrollContainer: ScrollContainer,
  previewEl: HTMLElement,
  focusLine: number,
  anchors: TypstSeekAnchor[]
): void {
  const containing = anchors.filter(
    (anchor) => focusLine >= anchor.start_line && focusLine <= anchor.end_line
  );
  if (containing.length) {
    const target = containing.reduce((best, anchor) => {
      if (!best) {
        return anchor;
      }
      const bestSpan = best.end_line - best.start_line;
      const span = anchor.end_line - anchor.start_line;
      return span < bestSpan ? anchor : best;
    }, containing[0]);
    scrollPreviewToTypstAnchor(scrollContainer, previewEl, target);
    return;
  }

  const before = anchors.reduce<TypstSeekAnchor | null>((best, anchor) => {
    if (anchor.end_line >= focusLine) {
      return best;
    }
    if (!best || anchor.end_line > best.end_line) {
      return anchor;
    }
    return best;
  }, null);
  const after = anchors.reduce<TypstSeekAnchor | null>((best, anchor) => {
    if (anchor.start_line <= focusLine) {
      return best;
    }
    if (!best || anchor.start_line < best.start_line) {
      return anchor;
    }
    return best;
  }, null);

  if (before && after) {
    scrollPreviewBetweenTypstAnchors(
      scrollContainer,
      previewEl,
      before,
      after,
      focusLine
    );
    return;
  }
  scrollPreviewToTypstAnchor(scrollContainer, previewEl, before || after);
}

function scrollPreviewToTypstAnchor(
  scrollContainer: ScrollContainer,
  previewEl: HTMLElement,
  anchor: TypstSeekAnchor | null
): void {
  const point = resolveTypstAnchorPoint(previewEl, anchor);
  if (point === null) {
    return;
  }
  const targetTop =
    getScrollContainerTop(scrollContainer) +
    (point - getScrollContainerViewportTop(scrollContainer)) -
    getScrollContainerViewportHeight(scrollContainer) * SEEK_VIEWPORT_ANCHOR;
  const maxTop = Math.max(
    0,
    getScrollContainerHeight(scrollContainer) -
      getScrollContainerViewportHeight(scrollContainer)
  );
  setScrollContainerTop(
    scrollContainer,
    Math.max(0, Math.min(maxTop, targetTop))
  );
}

function scrollPreviewBetweenTypstAnchors(
  scrollContainer: ScrollContainer,
  previewEl: HTMLElement,
  before: TypstSeekAnchor,
  after: TypstSeekAnchor,
  line: number
): void {
  const beforePoint = resolveTypstAnchorPoint(previewEl, before);
  const afterPoint = resolveTypstAnchorPoint(previewEl, after);
  if (beforePoint === null || afterPoint === null) {
    scrollPreviewToTypstAnchor(
      scrollContainer,
      previewEl,
      beforePoint === null ? after : before
    );
    return;
  }
  const lineSpan = Math.max(1, after.start_line - before.end_line);
  const progress = Math.max(
    0,
    Math.min(1, (line - before.end_line) / lineSpan)
  );
  const targetPoint = beforePoint + (afterPoint - beforePoint) * progress;
  const targetTop =
    getScrollContainerTop(scrollContainer) +
    (targetPoint - getScrollContainerViewportTop(scrollContainer)) -
    getScrollContainerViewportHeight(scrollContainer) * SEEK_VIEWPORT_ANCHOR;
  const maxTop = Math.max(
    0,
    getScrollContainerHeight(scrollContainer) -
      getScrollContainerViewportHeight(scrollContainer)
  );
  setScrollContainerTop(
    scrollContainer,
    Math.max(0, Math.min(maxTop, targetTop))
  );
}

function resolveTypstAnchorPoint(
  previewEl: HTMLElement,
  anchor: TypstSeekAnchor | null
): number | null {
  if (!anchor) {
    return null;
  }
  const page = previewEl.querySelector<HTMLElement>(
    `.typst-page[data-page="${anchor.page}"]`
  );
  if (!page) {
    return null;
  }
  const svg = page.querySelector<SVGSVGElement>("svg");
  if (!svg) {
    return page.getBoundingClientRect().top;
  }
  const viewBox = svg.viewBox.baseVal;
  const pageRect = page.getBoundingClientRect();
  const anchorY = Number(anchor.y);
  const ratio =
    viewBox && viewBox.height > 0 && Number.isFinite(anchorY)
      ? anchorY / viewBox.height
      : 0;
  return pageRect.top + pageRect.height * Math.max(0, Math.min(1, ratio));
}

/**
 * Collects preview elements that expose source line metadata.
 *
 * @param previewEl Root preview element.
 * @returns Ordered list of candidate elements for seek calculations.
 */
function collectMarkdownSeekCandidates(
  previewEl: HTMLElement
): MarkdownSeekCandidate[] {
  return [
    ...previewEl.querySelectorAll<HTMLElement>(
      "[data-source-start-line][data-source-end-line]"
    ),
  ]
    .map((node) => {
      const start = Number(node.dataset.sourceStartLine || 0);
      const end = Number(node.dataset.sourceEndLine || start);
      if (!start || !end) {
        return null;
      }
      return {
        node,
        start,
        end,
        // Deeper nodes are usually more specific matches when multiple elements
        // cover the same line span.
        depth: countNodeDepth(node),
      };
    })
    .filter(
      (candidate): candidate is MarkdownSeekCandidate => candidate !== null
    );
}

/**
 * Counts how deep a node is in the preview DOM tree.
 *
 * @param node Node to inspect.
 * @returns Parent depth, used to prefer more specific matches.
 */
function countNodeDepth(node: HTMLElement): number {
  let depth = 0;
  for (
    let current = node.parentElement;
    current;
    current = current.parentElement
  ) {
    depth += 1;
  }
  return depth;
}

/**
 * Scrolls the preview so a specific source line lands at the target reading position.
 *
 * @param scrollContainer Active scroll container for the preview.
 * @param candidate Best matching preview node.
 * @param line Source line to align.
 */
function scrollPreviewToLine(
  scrollContainer: ScrollContainer,
  candidate: MarkdownSeekCandidate | null,
  line: number
): void {
  if (!candidate) {
    return;
  }
  const nodeRect = candidate.node.getBoundingClientRect();
  const span = Math.max(1, candidate.end - candidate.start);
  const progress = Math.max(0, Math.min(1, (line - candidate.start) / span));
  const targetPoint = nodeRect.top + nodeRect.height * progress;
  const targetTop =
    getScrollContainerTop(scrollContainer) +
    (targetPoint - getScrollContainerViewportTop(scrollContainer)) -
    getScrollContainerViewportHeight(scrollContainer) * SEEK_VIEWPORT_ANCHOR;
  const maxTop = Math.max(
    0,
    getScrollContainerHeight(scrollContainer) -
      getScrollContainerViewportHeight(scrollContainer)
  );
  setScrollContainerTop(
    scrollContainer,
    Math.max(0, Math.min(maxTop, targetTop))
  );
}

/**
 * Interpolates between two neighboring Markdown blocks when the target line falls
 * between them rather than inside either block.
 *
 * @param scrollContainer Active scroll container for the preview.
 * @param before Closest block before the target line.
 * @param after Closest block after the target line.
 * @param line Source line to align.
 */
function scrollPreviewBetweenCandidates(
  scrollContainer: ScrollContainer,
  before: MarkdownSeekCandidate,
  after: MarkdownSeekCandidate,
  line: number
): void {
  const beforeRect = before.node.getBoundingClientRect();
  const afterRect = after.node.getBoundingClientRect();
  const lineSpan = Math.max(1, after.start - before.end);
  const progress = Math.max(0, Math.min(1, (line - before.end) / lineSpan));
  const beforePoint = beforeRect.top + beforeRect.height;
  const afterPoint = afterRect.top;
  const targetPoint = beforePoint + (afterPoint - beforePoint) * progress;
  const targetTop =
    getScrollContainerTop(scrollContainer) +
    (targetPoint - getScrollContainerViewportTop(scrollContainer)) -
    getScrollContainerViewportHeight(scrollContainer) * SEEK_VIEWPORT_ANCHOR;
  const maxTop = Math.max(
    0,
    getScrollContainerHeight(scrollContainer) -
      getScrollContainerViewportHeight(scrollContainer)
  );
  setScrollContainerTop(
    scrollContainer,
    Math.max(0, Math.min(maxTop, targetTop))
  );
}

function getScrollContainerTop(scrollContainer: ScrollContainer): number {
  if (!isRootScrollContainer(scrollContainer)) {
    return scrollContainer.scrollTop;
  }
  return window.scrollY || window.pageYOffset || scrollContainer.scrollTop || 0;
}

function setScrollContainerTop(
  scrollContainer: ScrollContainer,
  top: number
): void {
  if (!isRootScrollContainer(scrollContainer)) {
    scrollContainer.scrollTo({ top, behavior: "auto" });
    return;
  }
  window.scrollTo({ top, behavior: "auto" });
}

function getScrollContainerViewportTop(
  scrollContainer: ScrollContainer
): number {
  return isRootScrollContainer(scrollContainer)
    ? 0
    : scrollContainer.getBoundingClientRect().top;
}

function getScrollContainerViewportHeight(
  scrollContainer: ScrollContainer
): number {
  return isRootScrollContainer(scrollContainer)
    ? window.innerHeight
    : scrollContainer.clientHeight;
}

function getScrollContainerHeight(scrollContainer: ScrollContainer): number {
  if (!isRootScrollContainer(scrollContainer)) {
    return scrollContainer.scrollHeight;
  }
  return Math.max(
    scrollContainer.scrollHeight,
    document.body?.scrollHeight || 0,
    document.documentElement.scrollHeight
  );
}

function isRootScrollContainer(scrollContainer: ScrollContainer): boolean {
  const scrollingElement = document.scrollingElement;
  return (
    scrollContainer === scrollingElement ||
    scrollContainer === document.documentElement
  );
}
