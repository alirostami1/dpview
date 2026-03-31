import type {
  Elements,
  Page,
  PreviewRenderResult,
  State,
  TreeHandlers,
} from "./model";
import type { CurrentData, FrontMatter, LogEntry, TreeNode } from "./types";

/** Applies the current transient status and degraded-mode state to the settings panel. */
export function renderStatus(elements: Elements, state: State): void {
  const fragments: HTMLElement[] = [];

  if (state.statusMessage) {
    const line = document.createElement("div");
    line.className = "status-item";
    line.textContent = state.statusMessage;
    fragments.push(line);
  }

  const connection = document.createElement("div");
  connection.className = "status-item";
  connection.textContent = connectionLabel(state);
  if (state.connectionStatus === "degraded") {
    connection.classList.add("status-item-error");
  }
  fragments.push(connection);

  if (state.lastError) {
    const line = document.createElement("div");
    line.className = "status-item status-item-error";
    line.textContent = state.lastError;
    fragments.push(line);
  }

  elements.statusEl.replaceChildren(...fragments);
}

/** Renders the inline connection/retry banner shown above the preview. */
export function renderConnectionBanner(elements: Elements, state: State): void {
  const shouldShow =
    state.bootstrapFailed || state.connectionStatus === "degraded";
  elements.connectionBannerEl.classList.toggle("hidden", !shouldShow);
  elements.retryConnectionButton.classList.toggle("hidden", !shouldShow);
  if (!shouldShow) {
    elements.connectionMessageEl.textContent = "";
    return;
  }
  const parts = [connectionLabel(state)];
  if (state.lastError) {
    parts.push(state.lastError);
  }
  elements.connectionMessageEl.textContent = parts.join(" ");
}

function connectionLabel(state: State): string {
  if (state.connectionStatus === "live") {
    return "Connected.";
  }
  if (state.connectionStatus === "connecting") {
    if (state.connectionAttempts <= 0) {
      return "Connecting...";
    }
    const seconds = reconnectCountdownSeconds(state);
    return seconds > 0
      ? `Reconnecting (${state.connectionAttempts}) in ${seconds}s.`
      : `Retrying now (${state.connectionAttempts}).`;
  }
  const seconds = reconnectCountdownSeconds(state);
  return state.connectionAttempts > 0
    ? `Disconnected. Retry ${state.connectionAttempts} in ${seconds}s.`
    : `Disconnected. Retry in ${seconds}s.`;
}

function reconnectCountdownSeconds(state: State): number {
  if (state.reconnectAt <= 0) {
    return 0;
  }
  return Math.max(0, Math.ceil((state.reconnectAt - Date.now()) / 1000));
}

/** Applies sidebar visibility state to the shell DOM. */
export function renderSidebarShell(elements: Elements, state: State): void {
  elements.appEl.classList.toggle("sidebar-collapsed", state.sidebarCollapsed);
  elements.showSidebarButton.classList.toggle(
    "hidden",
    !state.sidebarCollapsed
  );
  elements.toggleSidebarButton.textContent = state.sidebarCollapsed
    ? "Show"
    : "Hide";
  elements.toggleSidebarButton.setAttribute(
    "aria-label",
    state.sidebarCollapsed ? "Show sidebar" : "Hide sidebar"
  );
  elements.toggleSidebarButton.setAttribute(
    "aria-expanded",
    String(!state.sidebarCollapsed)
  );
  if (!state.sidebarCollapsed) {
    elements.sidebarEl.removeAttribute("aria-hidden");
  } else {
    elements.sidebarEl.setAttribute("aria-hidden", "true");
  }
  elements.sidebarFilesViewEl.classList.toggle(
    "hidden",
    state.sidebarMode !== "files"
  );
  elements.sidebarSettingsViewEl.classList.toggle(
    "hidden",
    state.sidebarMode !== "settings"
  );
  elements.sidebarFilesTab.classList.toggle(
    "active",
    state.sidebarMode === "files"
  );
  elements.sidebarSettingsTab.classList.toggle(
    "active",
    state.sidebarMode === "settings"
  );
  elements.sidebarFilesTab.setAttribute(
    "aria-selected",
    String(state.sidebarMode === "files")
  );
  elements.sidebarSettingsTab.setAttribute(
    "aria-selected",
    String(state.sidebarMode === "settings")
  );
}

/** Switches which top-level page section is visible. */
export function setPage(elements: Elements, page: Page): void {
  elements.fileViewEl.classList.toggle("hidden", page !== "file");
  elements.notFoundViewEl.classList.toggle("hidden", page !== "not-found");
}

/** Renders the sidebar tree based on the current file list and search state. */
export function renderTree(
  elements: Elements,
  state: State,
  handlers: TreeHandlers
): void {
  elements.treeEl.setAttribute("role", "tree");
  const nodes = filteredNodes(state.tree, state.search);
  if (!nodes.length) {
    elements.treeEl.replaceChildren(
      document.createTextNode("No matching files.")
    );
    return;
  }
  const fragment = document.createDocumentFragment();
  const focusState = { assigned: false };
  renderTreeNodes(fragment, nodes, state, handlers, 1, "", focusState);
  elements.treeEl.replaceChildren(fragment);
  syncTreeTabStops(elements, state);
}

function filteredNodes(nodes: TreeNode[], search: string): TreeNode[] {
  if (!search) {
    return nodes;
  }
  const matchNode = (node: TreeNode): TreeNode | null => {
    const haystack = `${node.name} ${node.path || ""}`.toLowerCase();
    const isMatch = haystack.includes(search);
    if (node.children?.length) {
      const children = node.children
        .map(matchNode)
        .filter((value): value is TreeNode => value !== null);
      if (isMatch || children.length) {
        return { ...node, children };
      }
      return null;
    }
    return isMatch ? node : null;
  };
  return nodes
    .map(matchNode)
    .filter((value): value is TreeNode => value !== null);
}

function renderTreeNodes(
  container: DocumentFragment | HTMLElement,
  nodes: TreeNode[],
  state: State,
  handlers: TreeHandlers,
  level: number,
  parentPath: string,
  focusState: { assigned: boolean }
): void {
  for (const node of nodes) {
    const wrapper = document.createElement("div");
    wrapper.className = "tree-node";
    const nodePath = node.path || "";

    if (node.children?.length) {
      const open = state.expanded.has(nodePath) || Boolean(state.search);
      const toggle = document.createElement("button");
      toggle.type = "button";
      toggle.className = "tree-button tree-folder";
      toggle.style.paddingLeft = `${level * 0.75}rem`;
      toggle.textContent = `${open ? "▾" : "▸"} ${node.name}`;
      toggle.dataset.treeItem = "true";
      toggle.dataset.treeKind = "folder";
      toggle.dataset.treePath = nodePath;
      toggle.dataset.treeParentPath = parentPath;
      toggle.dataset.treeOpen = String(open);
      toggle.setAttribute("role", "treeitem");
      toggle.setAttribute("aria-expanded", String(open));
      toggle.setAttribute("aria-level", String(level));
      toggle.tabIndex = focusState.assigned ? -1 : 0;
      focusState.assigned = true;
      toggle.addEventListener("click", () => handlers.onToggleFolder(nodePath));
      wrapper.appendChild(toggle);

      if (open) {
        const children = document.createElement("div");
        children.className = "tree-children";
        children.setAttribute("role", "group");
        renderTreeNodes(
          children,
          node.children,
          state,
          handlers,
          level + 1,
          nodePath,
          focusState
        );
        wrapper.appendChild(children);
      }
    } else {
      const row = document.createElement("button");
      row.type = "button";
      row.className = "tree-button";
      row.style.paddingLeft = `${level * 0.75}rem`;
      row.textContent = node.name;
      row.title = nodePath || node.name;
      row.dataset.treeItem = "true";
      row.dataset.treeKind = "file";
      row.dataset.treePath = nodePath;
      row.dataset.treeParentPath = parentPath;
      row.setAttribute("role", "treeitem");
      row.setAttribute("aria-level", String(level));
      if (nodePath === state.current?.file?.path) {
        row.classList.add("selected");
        row.setAttribute("aria-current", "page");
        row.setAttribute("aria-selected", "true");
        row.tabIndex = 0;
        focusState.assigned = true;
      } else {
        row.setAttribute("aria-selected", "false");
        row.tabIndex = focusState.assigned ? -1 : 0;
        focusState.assigned = true;
      }
      row.addEventListener("click", () => handlers.onSelectFile(nodePath));
      wrapper.appendChild(row);
    }

    container.appendChild(wrapper);
  }
}

function syncTreeTabStops(elements: Elements, state: State): void {
  const items = [
    ...elements.treeEl.querySelectorAll<HTMLButtonElement>(
      "[data-tree-item='true']"
    ),
  ];
  if (!items.length) {
    return;
  }
  const selectedPath = state.current?.file?.path || "";
  const activeItem =
    items.find(
      (item) =>
        item.dataset.treeKind === "file" &&
        item.dataset.treePath === selectedPath
    ) || items[0];
  for (const item of items) {
    item.tabIndex = item === activeItem ? 0 : -1;
  }
}

/** Saves the front matter disclosure state before rerendering the preview DOM. */
export function rememberFrontMatterState(
  elements: Elements,
  state: State
): void {
  const panel =
    elements.previewEl.querySelector<HTMLDetailsElement>(".frontmatter-panel");
  if (panel) {
    state.frontMatterExpanded = panel.open;
  }
}

/** Reattaches front matter disclosure state listeners after preview rerendering. */
export function bindFrontMatterState(elements: Elements, state: State): void {
  const panel =
    elements.previewEl.querySelector<HTMLDetailsElement>(".frontmatter-panel");
  if (!panel) {
    return;
  }
  state.frontMatterExpanded = panel.open;
  panel.addEventListener("toggle", () => {
    state.frontMatterExpanded = panel.open;
  });
}

/** Renders the current preview shell and returns the server HTML insertion boundary. */
export function renderPreview(
  elements: Elements,
  state: State
): PreviewRenderResult {
  const current = resolvePreviewCurrent(state);
  const file = current?.file || null;
  const preview = current?.preview;
  const previewWrapperClass =
    file?.kind === "markdown"
      ? "preview-content markdown-preview"
      : file?.kind === "typst"
        ? "preview-content typst-preview"
        : "preview-content";

  if (preview?.status === "rendering") {
    elements.previewEl.className = "preview";
    elements.previewEl.replaceChildren(
      document.createTextNode(`Rendering ${file?.path || "file"}...`)
    );
    return { serverContentEl: null, markdownRoot: null };
  }

  if (preview?.error) {
    elements.previewEl.className = "preview";
    const wrapper = document.createElement("div");
    wrapper.className = previewWrapperClass;
    const strong = document.createElement("strong");
    strong.textContent = preview.error.message;
    wrapper.appendChild(strong);
    if (preview.error.detail) {
      const pre = document.createElement("pre");
      pre.textContent = preview.error.detail;
      wrapper.appendChild(pre);
    }
    elements.previewEl.replaceChildren(wrapper);
    return { serverContentEl: null, markdownRoot: null };
  }

  if (preview?.html) {
    elements.previewEl.className = "preview";
    const wrapper = createPreviewWrapper(
      current,
      state.settings.markdown_frontmatter_visible,
      state.frontMatterExpanded ?? state.settings.markdown_frontmatter_expanded
    );
    if (!wrapper) {
      elements.previewEl.className = "preview empty";
      elements.previewEl.replaceChildren(
        document.createTextNode("No preview available.")
      );
      return { serverContentEl: null, markdownRoot: null };
    }
    const serverContentEl = wrapper.querySelector<HTMLElement>(
      ".preview-server-html"
    );
    elements.previewEl.replaceChildren(wrapper);
    return {
      serverContentEl,
      markdownRoot: file?.kind === "markdown" ? serverContentEl : null,
    };
  }

  elements.previewEl.className = "preview empty";
  elements.previewEl.replaceChildren(
    document.createTextNode(
      file ? "No preview available." : "No file selected."
    )
  );
  return { serverContentEl: null, markdownRoot: null };
}

export function createPreviewWrapper(
  current: CurrentData | null,
  frontMatterVisible: boolean,
  frontMatterExpanded: boolean
): HTMLElement | null {
  const file = current?.file;
  const preview = current?.preview;
  if (!file || !preview?.html) {
    return null;
  }
  const wrapper = document.createElement("div");
  wrapper.className =
    file.kind === "markdown"
      ? "preview-content markdown-preview"
      : file.kind === "typst"
        ? "preview-content typst-preview"
        : "preview-content";

  if (file.kind === "markdown" && frontMatterVisible) {
    const panel = createFrontMatterPanel(
      preview.frontmatter,
      frontMatterExpanded
    );
    if (panel) {
      wrapper.appendChild(panel);
    }
  }

  const serverContentEl = document.createElement("div");
  serverContentEl.className = "preview-server-html";
  serverContentEl.innerHTML = preview.html;
  wrapper.appendChild(serverContentEl);
  return wrapper;
}

function resolvePreviewCurrent(state: State): State["current"] {
  const current = state.current;
  if (!current || current.preview.status !== "rendering") {
    return current;
  }

  const settled = state.lastSettledCurrent;
  if (!settled) {
    return current;
  }

  if ((settled.file?.path || "") !== (current.file?.path || "")) {
    return current;
  }

  return settled;
}

function createFrontMatterPanel(
  frontMatter: FrontMatter | undefined,
  open: boolean
): HTMLElement | null {
  if (!frontMatter?.entries?.length) {
    return null;
  }

  const details = document.createElement("details");
  details.className = "frontmatter-panel";
  details.open = open;

  const summary = document.createElement("summary");
  summary.append("Front matter");
  const meta = document.createElement("span");
  meta.className = "frontmatter-meta";
  meta.textContent = frontMatter.format || "yaml";
  summary.append(" ", meta);
  details.appendChild(summary);

  const wrap = document.createElement("div");
  wrap.className = "frontmatter-table-wrap";
  const table = document.createElement("table");
  table.className = "frontmatter-table";
  const body = document.createElement("tbody");
  for (const entry of frontMatter.entries) {
    const row = document.createElement("tr");
    const th = document.createElement("th");
    th.textContent = entry.key;
    const td = document.createElement("td");
    const code = document.createElement("code");
    code.textContent = entry.value;
    td.appendChild(code);
    row.append(th, td);
    body.appendChild(row);
  }
  table.appendChild(body);
  wrap.appendChild(table);
  details.appendChild(wrap);
  return details;
}

/** Renders backend health information into the settings page. */
export function renderHealth(elements: Elements, state: State): void {
  if (!state.health) {
    elements.healthEl.replaceChildren();
    return;
  }

  const fragment: HTMLElement[] = [];
  fragment.push(
    createStatusItem(`Status: ${state.health.status || "unknown"}`)
  );
  fragment.push(
    createStatusItem(
      `Watcher: ${state.health.watcher?.enabled ? "enabled" : "disabled"}`
    )
  );
  fragment.push(
    createStatusItem(
      `Max file size: ${formatBytes(state.health.limits.max_file_size_bytes)}`
    )
  );
  fragment.push(
    createStatusItem(
      `Render timeout: ${state.health.limits.render_timeout_ms} ms`
    )
  );
  for (const renderer of state.health.renderers || []) {
    fragment.push(
      createStatusItem(
        `${renderer.name || renderer.kind}: ${renderer.available ? "available" : "unavailable"}`
      )
    );
  }
  elements.healthEl.replaceChildren(...fragment);
}

/** Renders recent Go/runtime log entries inside settings. */
export function renderLogs(elements: Elements, state: State): void {
  const entries = state.logs?.entries || [];
  elements.clearLogsButton.disabled = entries.length === 0;
  elements.copyLogsButton.disabled = entries.length === 0;
  if (!entries.length) {
    elements.logsEl.value = "";
    elements.logsEl.placeholder = "No recent Go logs.";
    return;
  }
  elements.logsEl.placeholder = "";
  elements.logsEl.value = entries.map(formatLogEntry).join("\n\n");
}

function formatLogTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

function formatLogEntry(entry: LogEntry): string {
  const header = [
    `[${(entry.level || "info").toUpperCase()}]`,
    formatLogTime(entry.timestamp),
    entry.source || "app",
    entry.code || "event",
  ]
    .filter(Boolean)
    .join(" ");
  const lines = [header, entry.message || ""];
  const meta = [];
  if (entry.path) {
    meta.push(`path=${entry.path}`);
  }
  if (entry.context) {
    meta.push(`context=${entry.context}`);
  }
  if (meta.length) {
    lines.push(meta.join(" "));
  }
  if (entry.detail) {
    lines.push(entry.detail);
  }
  return lines.join("\n");
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "unknown";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KiB`;
  }
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
}

function createStatusItem(text: string): HTMLElement {
  const item = document.createElement("div");
  item.className = "status-item";
  item.textContent = text;
  return item;
}

/** Shows the not-found shell with a route-specific message. */
export function renderNotFound(elements: Elements, message: string): void {
  elements.notFoundMessageEl.textContent = message;
}
