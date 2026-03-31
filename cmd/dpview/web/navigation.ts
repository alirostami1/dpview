import type { Elements, State } from "./model";
import { renderNotFound, setPage } from "./render";
import { encodeAppPath, parseRoute } from "./routes";
import type { Route } from "./types";

interface NavigationControllerOptions {
  elements: Elements;
  state: State;
  fileExists: (path: string) => boolean;
  onSelectFile: (path: string) => void;
}

export interface NavigationController {
  bindWindowEvents: () => void;
  readCurrentRoute: () => Route;
  applyRoute: (route: Route) => void;
  navigateToCurrentFile: (options?: { replace?: boolean }) => void;
  navigateToFilePath: (
    path: string,
    options?: { replace?: boolean; apply?: boolean }
  ) => void;
  syncLocationPath: (path: string) => void;
  handleTreeKeydown: (event: KeyboardEvent) => void;
}

export function createNavigationController(
  options: NavigationControllerOptions
): NavigationController {
  const { elements, state } = options;

  function bindWindowEvents(): void {
    window.addEventListener("popstate", () => {
      applyRoute(readCurrentRoute());
    });
  }

  function readCurrentRoute(): Route {
    return parseRoute(window.location.pathname);
  }

  function applyRoute(route: Route): void {
    if (
      !route.path ||
      options.fileExists(route.path) ||
      state.files.length === 0
    ) {
      setPage(elements, "file");
      if (
        route.path &&
        state.current?.file?.path !== route.path &&
        options.fileExists(route.path)
      ) {
        options.onSelectFile(route.path);
      } else {
        requestAnimationFrame(() => elements.previewEl.focus());
      }
      return;
    }
    renderNotFound(elements, `No previewable file exists at "${route.path}".`);
    setPage(elements, "not-found");
    requestAnimationFrame(() => elements.goHomeButton.focus());
  }

  function navigateToCurrentFile(options: { replace?: boolean } = {}): void {
    navigateToFilePath(state.current?.file?.path || "", options);
  }

  function navigateToFilePath(
    path: string,
    routeOptions: { replace?: boolean; apply?: boolean } = {}
  ): void {
    const { replace = false, apply = true } = routeOptions;
    const url = new URL(window.location.href);
    url.pathname = encodeAppPath(path || "");
    url.search = "";
    if (replace) {
      window.history.replaceState({}, "", url);
    } else {
      window.history.pushState({}, "", url);
    }
    if (apply) {
      applyRoute(parseRoute(url.pathname));
    }
  }

  function syncLocationPath(path: string): void {
    const url = new URL(window.location.href);
    url.pathname = encodeAppPath(path || "");
    url.search = "";
    window.history.replaceState({}, "", url);
  }

  function handleTreeKeydown(event: KeyboardEvent): void {
    const target = event.target;
    if (
      !(target instanceof HTMLButtonElement) ||
      target.dataset.treeItem !== "true"
    ) {
      return;
    }

    const items = [
      ...elements.treeEl.querySelectorAll<HTMLButtonElement>(
        "[data-tree-item='true']"
      ),
    ];
    const index = items.indexOf(target);
    if (index === -1) {
      return;
    }

    const focusAt = (nextIndex: number) => {
      const item = items[nextIndex];
      if (item) {
        for (const treeItem of items) {
          treeItem.tabIndex = -1;
        }
        item.tabIndex = 0;
        item.focus();
      }
    };

    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        focusAt(Math.min(items.length - 1, index + 1));
        return;
      case "ArrowUp":
        event.preventDefault();
        focusAt(Math.max(0, index - 1));
        return;
      case "Home":
        event.preventDefault();
        focusAt(0);
        return;
      case "End":
        event.preventDefault();
        focusAt(items.length - 1);
        return;
      case "ArrowRight":
        if (
          target.dataset.treeKind === "folder" &&
          target.dataset.treeOpen === "false"
        ) {
          event.preventDefault();
          target.click();
        }
        return;
      case "ArrowLeft":
        if (
          target.dataset.treeKind === "folder" &&
          target.dataset.treeOpen === "true"
        ) {
          event.preventDefault();
          target.click();
          return;
        }
        if (target.dataset.treeParentPath) {
          const parent = elements.treeEl.querySelector<HTMLButtonElement>(
            `[data-tree-kind='folder'][data-tree-path='${CSS.escape(target.dataset.treeParentPath)}']`
          );
          if (parent) {
            event.preventDefault();
            parent.focus();
          }
        }
        return;
      case "Enter":
      case " ":
        event.preventDefault();
        target.click();
        return;
      default:
        return;
    }
  }

  return {
    bindWindowEvents,
    readCurrentRoute,
    applyRoute,
    navigateToCurrentFile,
    navigateToFilePath,
    syncLocationPath,
    handleTreeKeydown,
  };
}
