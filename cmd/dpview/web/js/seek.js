export function applyPreviewSeek(fileViewEl, previewEl, current, seek, settings) {
  const file = current?.file;
  const preview = current?.preview;
  if (!settings?.seek_enabled || !seek || !file || seek.path !== file.path || preview?.status !== "ready") {
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

function applyMarkdownSeek(fileViewEl, previewEl, seek) {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  if (!focusLine) {
    return;
  }
  const candidates = collectMarkdownSeekCandidates(previewEl);
  if (!candidates.length) {
    return;
  }

  const containing = candidates.filter((candidate) => (
    focusLine >= candidate.start && focusLine <= candidate.end
  ));

  if (containing.length) {
    const target = containing.reduce((best, candidate) => {
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
    .reduce((best, candidate) => (!best || candidate.end > best.end ? candidate : best), null);
  const after = candidates
    .filter((candidate) => candidate.start > focusLine)
    .reduce((best, candidate) => (!best || candidate.start < best.start ? candidate : best), null);

  if (before && after) {
    scrollPreviewBetweenCandidates(fileViewEl, before, after, focusLine);
    return;
  }

  scrollPreviewToLine(fileViewEl, before || after, focusLine);
}

function applyTypstSeek(fileViewEl, seek, preview) {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  const totalLines = Number(preview?.source_line_count || 0);
  if (!focusLine || !totalLines) {
    return;
  }
  const scrollRange = fileViewEl.scrollHeight - fileViewEl.clientHeight;
  if (scrollRange <= 0) {
    return;
  }
  const progress = totalLines <= 1 ? 0 : Math.max(0, Math.min(1, (focusLine - 1) / (totalLines - 1)));
  fileViewEl.scrollTo({ top: progress * scrollRange, behavior: "auto" });
}

function collectMarkdownSeekCandidates(previewEl) {
  return [...previewEl.querySelectorAll("[data-source-start-line][data-source-end-line]")]
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
        depth: countNodeDepth(node),
      };
    })
    .filter(Boolean);
}

function countNodeDepth(node) {
  let depth = 0;
  for (let current = node.parentElement; current; current = current.parentElement) {
    depth += 1;
  }
  return depth;
}

function scrollPreviewToLine(fileViewEl, candidate, line) {
  if (!candidate) {
    return;
  }
  const containerRect = fileViewEl.getBoundingClientRect();
  const nodeRect = candidate.node.getBoundingClientRect();
  const span = Math.max(1, candidate.end - candidate.start);
  const progress = Math.max(0, Math.min(1, (line - candidate.start) / span));
  const targetPoint = nodeRect.top + (nodeRect.height * progress);
  const targetTop = fileViewEl.scrollTop + (targetPoint - containerRect.top) - (fileViewEl.clientHeight * 0.32);
  const maxTop = Math.max(0, fileViewEl.scrollHeight - fileViewEl.clientHeight);
  fileViewEl.scrollTo({
    top: Math.max(0, Math.min(maxTop, targetTop)),
    behavior: "auto",
  });
}

function scrollPreviewBetweenCandidates(fileViewEl, before, after, line) {
  const containerRect = fileViewEl.getBoundingClientRect();
  const beforeRect = before.node.getBoundingClientRect();
  const afterRect = after.node.getBoundingClientRect();
  const lineSpan = Math.max(1, after.start - before.end);
  const progress = Math.max(0, Math.min(1, (line - before.end) / lineSpan));
  const beforePoint = beforeRect.top + beforeRect.height;
  const afterPoint = afterRect.top;
  const targetPoint = beforePoint + ((afterPoint - beforePoint) * progress);
  const targetTop = fileViewEl.scrollTop + (targetPoint - containerRect.top) - (fileViewEl.clientHeight * 0.32);
  const maxTop = Math.max(0, fileViewEl.scrollHeight - fileViewEl.clientHeight);
  fileViewEl.scrollTo({
    top: Math.max(0, Math.min(maxTop, targetTop)),
    behavior: "auto",
  });
}
