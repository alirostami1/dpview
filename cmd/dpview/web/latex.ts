type LatexRuntime = {
  renderMarkdownMath: (container: Element | null) => void;
};

export type LatexRenderResult =
  | { kind: "disabled" }
  | { kind: "no-container" }
  | { kind: "no-math" }
  | { kind: "rendered" }
  | { kind: "failed"; error: Error };

type LoaderState = "idle" | "loading" | "loaded" | "failed";

let runtimePromise: Promise<LatexRuntime> | null = null;
let stylesheetPromise: Promise<void> | null = null;
let loaderState: LoaderState = "idle";
let loaderError: Error | null = null;
let loggedNoMathSkip = false;

function debugLog(message: string, detail?: unknown): void {
  if (detail === undefined) {
    console.debug(`[dpview] ${message}`);
    return;
  }
  console.debug(`[dpview] ${message}`, detail);
}

function normalizeError(error: unknown, fallbackMessage: string): Error {
  return error instanceof Error ? error : new Error(fallbackMessage);
}

function previewHasLatexMath(container: Element): boolean {
  return container.querySelector(".markdown-math-block") !== null;
}

function ensureLatexStylesheet(): Promise<void> {
  if (stylesheetPromise) {
    return stylesheetPromise;
  }
  const existing = document.querySelector<HTMLLinkElement>(
    "link[data-dpview-latex='true']"
  );
  if (existing) {
    stylesheetPromise = Promise.resolve();
    return stylesheetPromise;
  }
  stylesheetPromise = new Promise((resolve, reject) => {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "/katex-style.css";
    link.dataset.dpviewLatex = "true";
    link.addEventListener(
      "load",
      () => {
        debugLog("KaTeX stylesheet loaded.");
        resolve();
      },
      { once: true }
    );
    link.addEventListener(
      "error",
      () => {
        reject(new Error("Failed to load KaTeX stylesheet."));
      },
      { once: true }
    );
    document.head.appendChild(link);
  });
  return stylesheetPromise;
}

async function ensureLatexRuntime(): Promise<LatexRuntime> {
  if (loaderState === "loaded" && runtimePromise) {
    return runtimePromise;
  }
  if (loaderState === "failed" && loaderError) {
    throw loaderError;
  }
  if (runtimePromise) {
    return runtimePromise;
  }

  loaderState = "loading";
  debugLog("Loading KaTeX runtime.");
  runtimePromise = (async () => {
    try {
      await ensureLatexStylesheet();
      const runtimeURL = "/katex-runtime.js";
      const runtime = await (import(runtimeURL) as Promise<LatexRuntime>);
      loaderState = "loaded";
      debugLog("KaTeX runtime loaded.");
      return runtime;
    } catch (error) {
      const normalized = normalizeError(error, "Failed to load KaTeX runtime.");
      loaderState = "failed";
      loaderError = normalized;
      debugLog("KaTeX runtime failed to load.", normalized);
      throw normalized;
    }
  })();

  return runtimePromise;
}

export async function renderMarkdownMath(
  container: Element | null,
  enabled: boolean
): Promise<LatexRenderResult> {
  if (!enabled) {
    return { kind: "disabled" };
  }
  if (!container) {
    return { kind: "no-container" };
  }
  if (!previewHasLatexMath(container)) {
    if (!loggedNoMathSkip) {
      debugLog(
        "Skipping KaTeX load because the preview contains no math placeholders."
      );
      loggedNoMathSkip = true;
    }
    return { kind: "no-math" };
  }

  try {
    const runtime = await ensureLatexRuntime();
    runtime.renderMarkdownMath(container);
    return { kind: "rendered" };
  } catch (error) {
    return {
      kind: "failed",
      error: normalizeError(error, "Failed to load KaTeX runtime."),
    };
  }
}
