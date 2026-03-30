import type { CurrentData, Preview, SeekData, Settings } from "./types";

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

/**
 * Applies editor seek state to the current preview when possible.
 *
 * @param fileViewEl Scroll container for the current file view.
 * @param previewEl Root preview element.
 * @param current Current file/preview snapshot.
 * @param seek Current seek position snapshot.
 * @param settings Active settings snapshot.
 */
export function applyPreviewSeek(
  fileViewEl: HTMLElement,
  previewEl: HTMLElement,
  current: CurrentData | null,
  seek: SeekData | null,
  settings: Settings,
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
    applyMarkdownSeek(fileViewEl, previewEl, seek);
    return;
  }
  if (file.kind === "typst") {
    applyTypstSeek(fileViewEl, seek, preview);
  }
}

/**
 * Aligns a Markdown preview to the best matching source line block.
 *
 * @param fileViewEl Scroll container for the preview.
 * @param previewEl Root preview element.
 * @param seek Current seek position snapshot.
 */
function applyMarkdownSeek(
  fileViewEl: HTMLElement,
  previewEl: HTMLElement,
  seek: SeekData,
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
    (candidate) => focusLine >= candidate.start && focusLine <= candidate.end,
  );

  if (containing.length) {
    // Prefer the narrowest matching block so large containers do not win over the
    // specific paragraph/list item/code block the editor is currently focused on.
    const target = containing.reduce<MarkdownSeekCandidate | null>((best, candidate) => {
      if (!best) {
        return candidate;
      }
      const bestSpan = best.end - best.start;
      const candidateSpan = candidate.end - candidate.start;
      if (candidateSpan !== bestSpan) {
        return candidateSpan < bestSpan ? candidate : best;
      }
      return candidate.depth > best.depth ? candidate : best;
    }, null);
    scrollPreviewToLine(fileViewEl, target, focusLine);
    return;
  }

  const before = candidates
    .filter((candidate) => candidate.end < focusLine)
    .reduce<MarkdownSeekCandidate | null>(
      (best, candidate) => (!best || candidate.end > best.end ? candidate : best),
      null,
    );
  const after = candidates
    .filter((candidate) => candidate.start > focusLine)
    .reduce<MarkdownSeekCandidate | null>(
      (best, candidate) => (!best || candidate.start < best.start ? candidate : best),
      null,
    );

  if (before && after) {
    scrollPreviewBetweenCandidates(fileViewEl, before, after, focusLine);
    return;
  }

  scrollPreviewToLine(fileViewEl, before || after, focusLine);
}

/**
 * Aligns a Typst preview proportionally using source line progress.
 *
 * @param fileViewEl Scroll container for the preview.
 * @param seek Current seek position snapshot.
 * @param preview Current preview data.
 */
function applyTypstSeek(
  fileViewEl: HTMLElement,
  seek: SeekData,
  preview: Preview,
): void {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  const totalLines = Number(preview.source_line_count || 0);
  if (!focusLine || !totalLines) {
    return;
  }
  const scrollRange = fileViewEl.scrollHeight - fileViewEl.clientHeight;
  if (scrollRange <= 0) {
    return;
  }
  const progress =
    totalLines <= 1 ? 0 : Math.max(0, Math.min(1, (focusLine - 1) / (totalLines - 1)));
  fileViewEl.scrollTo({ top: progress * scrollRange, behavior: "auto" });
}

/**
 * Collects preview elements that expose source line metadata.
 *
 * @param previewEl Root preview element.
 * @returns Ordered list of candidate elements for seek calculations.
 */
function collectMarkdownSeekCandidates(previewEl: HTMLElement): MarkdownSeekCandidate[] {
  return [...previewEl.querySelectorAll<HTMLElement>("[data-source-start-line][data-source-end-line]")]
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
    .filter((candidate): candidate is MarkdownSeekCandidate => candidate !== null);
}

/**
 * Counts how deep a node is in the preview DOM tree.
 *
 * @param node Node to inspect.
 * @returns Parent depth, used to prefer more specific matches.
 */
function countNodeDepth(node: HTMLElement): number {
  let depth = 0;
  for (let current = node.parentElement; current; current = current.parentElement) {
    depth += 1;
  }
  return depth;
}

/**
 * Scrolls the preview so a specific source line lands at the target reading position.
 *
 * @param fileViewEl Scroll container for the preview.
 * @param candidate Best matching preview node.
 * @param line Source line to align.
 */
function scrollPreviewToLine(
  fileViewEl: HTMLElement,
  candidate: MarkdownSeekCandidate | null,
  line: number,
): void {
  if (!candidate) {
    return;
  }
  const containerRect = fileViewEl.getBoundingClientRect();
  const nodeRect = candidate.node.getBoundingClientRect();
  const span = Math.max(1, candidate.end - candidate.start);
  const progress = Math.max(0, Math.min(1, (line - candidate.start) / span));
  const targetPoint = nodeRect.top + (nodeRect.height * progress);
  const targetTop =
    fileViewEl.scrollTop + (targetPoint - containerRect.top) - (fileViewEl.clientHeight * 0.32);
  const maxTop = Math.max(0, fileViewEl.scrollHeight - fileViewEl.clientHeight);
  fileViewEl.scrollTo({
    top: Math.max(0, Math.min(maxTop, targetTop)),
    behavior: "auto",
  });
}

/**
 * Interpolates between two neighboring Markdown blocks when the target line falls
 * between them rather than inside either block.
 *
 * @param fileViewEl Scroll container for the preview.
 * @param before Closest block before the target line.
 * @param after Closest block after the target line.
 * @param line Source line to align.
 */
function scrollPreviewBetweenCandidates(
  fileViewEl: HTMLElement,
  before: MarkdownSeekCandidate,
  after: MarkdownSeekCandidate,
  line: number,
): void {
  const containerRect = fileViewEl.getBoundingClientRect();
  const beforeRect = before.node.getBoundingClientRect();
  const afterRect = after.node.getBoundingClientRect();
  const lineSpan = Math.max(1, after.start - before.end);
  const progress = Math.max(0, Math.min(1, (line - before.end) / lineSpan));
  const beforePoint = beforeRect.top + beforeRect.height;
  const afterPoint = afterRect.top;
  const targetPoint = beforePoint + ((afterPoint - beforePoint) * progress);
  const targetTop =
    fileViewEl.scrollTop + (targetPoint - containerRect.top) - (fileViewEl.clientHeight * 0.32);
  const maxTop = Math.max(0, fileViewEl.scrollHeight - fileViewEl.clientHeight);
  fileViewEl.scrollTo({
    top: Math.max(0, Math.min(maxTop, targetTop)),
    behavior: "auto",
  });
}
