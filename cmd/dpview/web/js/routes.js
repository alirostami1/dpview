export function encodeAppPath(path) {
  if (!path) {
    return "/";
  }
  return `/${path.split("/").map((part) => encodeURIComponent(part)).join("/")}`;
}

export function parseRoute(pathname, search = "") {
  const params = new URLSearchParams(search);
  const settingsOpen = params.get("settings") === "open";
  const decodedPath = pathname
    .split("/")
    .filter(Boolean)
    .map((part) => decodeURIComponent(part));

  if (settingsOpen) {
    return { kind: "settings" };
  }
  if (decodedPath.length === 0) {
    return { kind: "file", path: "" };
  }
  return { kind: "file", path: decodedPath.join("/") };
}

export function isSettingsRoute(locationLike = window.location) {
  return parseRoute(locationLike.pathname, locationLike.search).kind === "settings";
}
