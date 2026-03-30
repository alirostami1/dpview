import { z } from "zod";

export const storedThemes = ["system", "dark", "light"] as const;
export const resolvedThemes = ["dark", "light"] as const;
export const previewThemes = ["default", "github", "notion", "paper"] as const;

export type StoredTheme = (typeof storedThemes)[number];
export type ResolvedTheme = (typeof resolvedThemes)[number];
export type PreviewTheme = (typeof previewThemes)[number];

export const storedThemeSchema = z.enum(storedThemes);
export const resolvedThemeSchema = z.enum(resolvedThemes);
export const previewThemeSchema = z.enum(previewThemes);

export type Route = { kind: "file"; path: string };

export interface SettingsPayload {
    auto_refresh_paused: boolean;
    sidebar_collapsed: boolean;
    editor_file_sync_enabled: boolean;
    seek_enabled: boolean;
    typst_preview_theme: boolean;
    markdown_frontmatter_visible: boolean;
    markdown_frontmatter_expanded: boolean;
    markdown_frontmatter_title: boolean;
    theme: ResolvedTheme;
    preview_theme: PreviewTheme;
}

export const settingsPayloadSchema: z.ZodType<SettingsPayload> = z.object({
    auto_refresh_paused: z.boolean(),
    sidebar_collapsed: z.boolean(),
    editor_file_sync_enabled: z.boolean(),
    seek_enabled: z.boolean(),
    typst_preview_theme: z.boolean(),
    markdown_frontmatter_visible: z.boolean(),
    markdown_frontmatter_expanded: z.boolean(),
    markdown_frontmatter_title: z.boolean(),
    theme: resolvedThemeSchema,
    preview_theme: previewThemeSchema,
});

export const expandedPathsStorageSchema = z.array(z.string());
export const currentPathStorageSchema = z.string();
export const searchStorageSchema = z.string();
