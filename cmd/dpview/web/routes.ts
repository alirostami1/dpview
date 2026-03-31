import type { Route } from "./types";

export function encodeAppPath(path: string): string {
  if (!path) {
    return "/";
  }
  return `/${path
    .split("/")
    .map((part) => encodeURIComponent(part))
    .join("/")}`;
}

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
