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
 * @returns Parsed route understood by the application.
 */
export function parseRoute(pathname: string): Route {
  const decodedPath = pathname
    .split("/")
    .filter(Boolean)
    .map((part) => decodeURIComponent(part));

  if (decodedPath.length === 0) {
    return { kind: "file", path: "" };
  }
  return { kind: "file", path: decodedPath.join("/") };
}
