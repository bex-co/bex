/*
 * Copyright 2026.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// One UTC wire format: ISO-8601 with millisecond precision and a `Z` suffix,
// identical to `Date.prototype.toISOString()` (e.g. `2026-08-19T23:06:00.000Z`).
//
// Canonical field: top-level `at` on every published stream chunk. The AI SDK
// copies `providerMetadata` onto assembled text/reasoning/tool parts, so the
// same instant is also stored at `providerMetadata.bex.at` (and `endAt` on a
// closed text/reasoning block) — never a second clock, never a rewrite after
// first publication. Deltas omit providerMetadata so useChat keeps the start.

export const SOURCE_TIME_FIELD = "at";
export const SOURCE_TIME_PROVIDER = "bex";

const ISO_UTC = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(\.\d{1,9})?Z$/;

const DELTA_TYPES = new Set(["text-delta", "reasoning-delta"]);

export function utcNow(): string {
  return new Date().toISOString();
}

export function isUtcTimestamp(value: unknown): value is string {
  if (typeof value !== "string" || !ISO_UTC.test(value)) return false;
  return Number.isFinite(Date.parse(value));
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

function bexMeta(
  part: Record<string, unknown>,
): Record<string, unknown> | undefined {
  return asRecord(asRecord(part.providerMetadata)?.[SOURCE_TIME_PROVIDER]);
}

/** The already-assigned publication instant, or undefined when the part is untimed. */
export function existingSourceTime(
  part: Record<string, unknown>,
): string | undefined {
  const at = part[SOURCE_TIME_FIELD];
  return isUtcTimestamp(at) ? at : undefined;
}

/**
 * Stamp `at` once at the publication boundary. An existing valid timestamp is
 * preserved (retry/replay); missing or invalid optional timing is replaced.
 * Text/reasoning deltas get only the top-level field so assembled parts keep
 * the start instant from `*-start`.
 */
export function stampSourceTimestamp<T extends Record<string, unknown>>(
  part: T,
  atNow?: string,
): T {
  const existing = existingSourceTime(part);
  const at = existing ?? atNow ?? utcNow();
  const type = typeof part.type === "string" ? part.type : "";
  const skipMeta = DELTA_TYPES.has(type);
  const needsTop = part[SOURCE_TIME_FIELD] !== at;
  const needsMeta =
    !skipMeta && !isUtcTimestamp(bexMeta(part)?.[SOURCE_TIME_FIELD]);
  if (!needsTop && !needsMeta) return part;

  const next: Record<string, unknown> = { ...part, [SOURCE_TIME_FIELD]: at };
  if (skipMeta || !needsMeta) return next as T;

  const providerMetadata = { ...asRecord(part.providerMetadata) };
  const bex = { ...asRecord(providerMetadata[SOURCE_TIME_PROVIDER]) };
  bex[SOURCE_TIME_FIELD] = at;
  providerMetadata[SOURCE_TIME_PROVIDER] = bex;
  next.providerMetadata = providerMetadata;
  return next as T;
}
