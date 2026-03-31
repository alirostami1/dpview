import type { ZodType } from "zod";

export const STORAGE = {
    expanded: "dpview.expanded",
    currentPath: "dpview.currentPath",
    search: "dpview.search",
    theme: "dpview.theme",
} as const;

/** Reads a string value from localStorage with schema validation and fallback. */
export function readStoredString<T extends string>(
    key: string,
    schema: ZodType<T>,
    fallback: T,
): T {
    const rawValue = localStorage.getItem(key);
    const result = schema.safeParse(rawValue);
    return result.success ? result.data : fallback;
}

/** Reads a JSON value from localStorage with schema validation and fallback. */
export function readStoredJSON<T>(
    key: string,
    schema: ZodType<T>,
    fallback: T,
): T {
    const rawValue = localStorage.getItem(key);
    if (!rawValue) {
        return fallback;
    }
    try {
        const parsed = JSON.parse(rawValue);
        const result = schema.safeParse(parsed);
        return result.success ? result.data : fallback;
    } catch {
        return fallback;
    }
}
