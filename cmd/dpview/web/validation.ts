import { z, ZodError, type ZodType } from "zod";
import type { ApiResult } from "./types";
import {
  currentPathStorageSchema,
  expandedPathsStorageSchema,
  previewThemeSchema,
  searchStorageSchema,
  settingsPayloadSchema,
  storedThemeSchema,
} from "./local-types";
import {
  currentDataSchema,
  filesDataSchema,
  healthDataSchema,
  logDataSchema,
  seekDataSchema,
  settingsDataSchema,
  apiErrorSchema,
} from "./generated/contracts";

export {
  currentDataSchema,
  currentPathStorageSchema,
  expandedPathsStorageSchema,
  filesDataSchema,
  healthDataSchema,
  logDataSchema,
  previewThemeSchema,
  searchStorageSchema,
  seekDataSchema,
  settingsDataSchema,
  settingsPayloadSchema,
  storedThemeSchema,
};

export function formatZodError(error: ZodError): string {
  return error.issues
    .map((issue) => {
      const path = issue.path.length > 0 ? issue.path.join(".") : "root";
      return `${path}: ${issue.message}`;
    })
    .join("; ");
}

export function parseApiEnvelope<T>(
  payload: unknown,
  endpoint: string,
  dataSchema: ZodType<T>
): ApiResult<T> {
  const envelopeResult = apiEnvelopeSchema(dataSchema).safeParse(payload);
  if (!envelopeResult.success) {
    return {
      ok: false,
      error: {
        code: "invalid_response",
        message: `Invalid response from ${endpoint}`,
        detail: formatZodError(envelopeResult.error),
      },
    };
  }
  return envelopeResult.data;
}

export function apiEnvelopeSchema<T>(dataSchema: ZodType<T>) {
  return z.union([
    z
      .object({
        ok: z.literal(true),
        data: dataSchema,
      })
      .passthrough(),
    z
      .object({
        ok: z.literal(false),
        error: apiErrorSchema,
      })
      .passthrough(),
  ]);
}

export function eventPayloadSchema<T>(dataSchema: ZodType<T>) {
  return z
    .object({
      event_id: z.number().int(),
      version: z.number().int(),
      data: dataSchema,
    })
    .passthrough();
}

export function parseEventData<T>(
  event: Event,
  label: string,
  dataSchema: ZodType<T>
): T {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") {
    throw new Error(`Invalid ${label} event payload`);
  }
  let parsedJSON: unknown;
  try {
    parsedJSON = JSON.parse(event.data);
  } catch {
    throw new Error(`Invalid ${label} event payload`);
  }
  const result = eventPayloadSchema(dataSchema).safeParse(parsedJSON);
  if (!result.success) {
    throw new Error(
      `Invalid ${label} event payload: ${formatZodError(result.error)}`
    );
  }
  return result.data.data;
}
