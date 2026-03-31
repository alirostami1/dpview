import type { ZodType } from "zod";
import type { ApiResult } from "./types";
import { parseApiEnvelope } from "./validation";

/** Performs a fetch against the DPview API envelope format. */
export async function apiFetch<T>(
  path: string,
  dataSchema: ZodType<T>,
  options: RequestInit = {}
): Promise<ApiResult<T>> {
  let response: Response;
  try {
    response = await fetch(path, options);
  } catch (error) {
    return {
      ok: false,
      error: {
        code: "network_error",
        message: `Request failed for ${path}`,
        detail: error instanceof Error ? error.message : String(error),
      },
    };
  }

  const rawPayload = await response.json().catch(() => null);
  const result = parseApiEnvelope(rawPayload, path, dataSchema);
  if (!response.ok && result.ok) {
    return {
      ok: false,
      error: {
        code: "http_error",
        message: `Request failed with status ${response.status}`,
      },
    };
  }
  return result;
}
