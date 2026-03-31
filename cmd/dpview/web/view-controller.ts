import morphdom from "morphdom";
import type { Elements, State } from "./model";
import {
  bindFrontMatterState,
  createPreviewWrapper,
  renderHealth,
  renderLogs,
  renderPreview,
  renderStatus,
  renderTree,
  rememberFrontMatterState,
} from "./render";
import { applyPreviewSeek } from "./seek";
import { STORAGE } from "./storage";
import type { CurrentData } from "./types";

interface ViewControllerOptions {
  elements: Elements;
  state: State;
  onSelectFile: (path: string) => void;
  onRenderMarkdownMath: (container: Element | null) => void;
}

export interface ViewController {
  renderTreeUI: () => void;
  renderPreviewUI: () => void;
  renderSettingsUI: () => void;
  morphMarkdownPreview: (
    previous: CurrentData | null,
    current: CurrentData | null
  ) => boolean;
  queueApplySeek: () => void;
}

export function createViewController(
  options: ViewControllerOptions
): ViewController {
  const { elements, state } = options;

  function renderTreeUI(): void {
    renderTree(elements, state, {
      onToggleFolder: (path) => {
        if (state.expanded.has(path)) {
          state.expanded.delete(path);
        } else {
          state.expanded.add(path);
        }
        localStorage.setItem(
          STORAGE.expanded,
          JSON.stringify([...state.expanded])
        );
        renderTreeUI();
      },
      onSelectFile: (path) => {
        options.onSelectFile(path);
      },
    });
  }

  function renderPreviewUI(): void {
    rememberFrontMatterState(elements, state);
    const renderResult = renderPreview(elements, state);
    elements.previewEl.setAttribute(
      "aria-busy",
      String(state.current?.preview.status === "rendering")
    );
    bindFrontMatterState(elements, state);
    if (renderResult.markdownRoot) {
      options.onRenderMarkdownMath(renderResult.markdownRoot);
    }
    if (renderResult.serverContentEl) {
      queueApplySeek();
    }
  }

  function morphMarkdownPreview(
    previous: CurrentData | null,
    current: CurrentData | null
  ): boolean {
    if (
      !previous ||
      !current ||
      previous.file?.path !== current.file?.path ||
      previous.file?.kind !== "markdown" ||
      current.file?.kind !== "markdown" ||
      current.preview.status !== "ready" ||
      !current.preview.html ||
      current.preview.error
    ) {
      return false;
    }

    const existingWrapper = elements.previewEl.querySelector<HTMLElement>(
      ".preview-content.markdown-preview"
    );
    if (!existingWrapper) {
      return false;
    }

    rememberFrontMatterState(elements, state);
    const nextWrapper = createPreviewWrapper(
      current,
      state.settings.markdown_frontmatter_visible,
      state.frontMatterExpanded ?? state.settings.markdown_frontmatter_expanded
    );
    if (!nextWrapper) {
      return false;
    }

    morphdom(existingWrapper, nextWrapper);
    elements.previewEl.className = "preview";
    elements.previewEl.setAttribute("aria-busy", "false");
    bindFrontMatterState(elements, state);

    const markdownRoot = existingWrapper.querySelector<HTMLElement>(
      ".preview-server-html"
    );
    if (markdownRoot) {
      options.onRenderMarkdownMath(markdownRoot);
    }
    return true;
  }

  function renderSettingsUI(): void {
    renderStatus(elements, state);
    renderHealth(elements, state);
    renderLogs(elements, state);
    elements.themeSelect.value = state.theme;
    elements.previewThemeSelect.value = state.previewTheme;
    elements.pauseRefreshInput.checked = state.settings.auto_refresh_paused;
    elements.editorFileSyncInput.checked =
      state.settings.editor_file_sync_enabled;
    elements.liveBufferPreviewInput.checked =
      state.settings.live_buffer_preview_enabled;
    elements.seekEnabledInput.checked = state.settings.seek_enabled;
    elements.latexEnabledInput.checked = state.settings.latex_enabled;
    elements.typstPreviewThemeInput.checked =
      state.settings.typst_preview_theme;
    elements.markdownFrontMatterVisibleInput.checked =
      state.settings.markdown_frontmatter_visible;
    elements.markdownFrontMatterExpandedInput.checked =
      state.settings.markdown_frontmatter_expanded;
    elements.markdownFrontMatterTitleInput.checked =
      state.settings.markdown_frontmatter_title;
  }

  function queueApplySeek(): void {
    if (state.pendingSeekFrame) {
      cancelAnimationFrame(state.pendingSeekFrame);
    }
    state.pendingSeekFrame = requestAnimationFrame(() => {
      state.pendingSeekFrame = 0;
      applyPreviewSeek(
        elements.fileViewEl,
        elements.previewEl,
        state.current,
        state.seek,
        state.settings
      );
    });
  }

  return {
    renderTreeUI,
    renderPreviewUI,
    renderSettingsUI,
    morphMarkdownPreview,
    queueApplySeek,
  };
}
