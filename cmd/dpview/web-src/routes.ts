import type { Route } from "./types";

/**
 * Converts an internal relative file path into the browser route path used by DPview.
 *
 * @param path Relative file path from the backend.
 * @returns URL-safe app route path.
 */
export function encodeAppPath(path: string): string {
  if (!path) {
    return "/";
  }
  return `/${path.split("/").map((part) => encodeURIComponent(part)).join("/")}`;
}

/**
 * Parses the current browser location into a DPview route object.
 *
 * @param pathname Browser pathname component.
 * @param search Browser search/query component.
 * @returns Parsed route understood by the application.
 */
export function parseRoute(pathname: string, search = ""): Route {
  const params = new URLSearchParams(search);
  const settingsOpen = params.get("settings") === "open";
  const decodedPath = pathname
    .split("/")
    .filter(Boolean)
    .map((part) => decodeURIComponent(part));

  if (settingsOpen) {
    // The settings screen is a query-driven overlay on top of the current file route.
    return { kind: "settings" };
  }
  if (decodedPath.length === 0) {
    return { kind: "file", path: "" };
  }
  return { kind: "file", path: decodedPath.join("/") };
}

/**
 * Checks whether a location points at the settings overlay route.
 *
 * @param locationLike Location-like object to inspect.
 * @returns `true` when the location opens settings.
 */
export function isSettingsRoute(
  locationLike: Pick<Location, "pathname" | "search"> = window.location,
): boolean {
  return parseRoute(locationLike.pathname, locationLike.search).kind === "settings";
}
