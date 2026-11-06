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

// Session secret redaction (ADR098 D6). bex-api injects the session's secrets
// as plain env vars and names them in BEX_AGENT_SECRET_NAMES; the driver scrubs
// their values (and their cheap encodings) from every part before it is
// published, so the live stream, the gateway's durable tee and the turn log
// never carry a value. Values shorter than minRedactedLength are not scrubbed:
// replacing every "abc" in a transcript would destroy it and protect nothing.

export const minRedactedLength = 4;

interface Needle {
  text: string;
  label: string;
}

type Part = Record<string, unknown>;

// secretValuesFromEnv resolves BEX_AGENT_SECRET_NAMES (comma-separated) to the
// name → value pairs the driver must scrub. Unknown or empty names are skipped.
export function secretValuesFromEnv(
  env: NodeJS.ProcessEnv = process.env,
): Map<string, string> {
  const out = new Map<string, string>();
  for (const raw of (env.BEX_AGENT_SECRET_NAMES || "").split(",")) {
    const name = raw.trim();
    const value = name ? env[name] : undefined;
    if (value) out.set(name, value);
  }
  return out;
}

function variants(value: string): string[] {
  const bytes = Buffer.from(value, "utf8");
  const std = bytes.toString("base64");
  const all = [
    value,
    std,
    std.replace(/=+$/, ""),
    bytes.toString("base64url"),
    encodeURIComponent(value),
    JSON.stringify(value).slice(1, -1),
  ];
  return [...new Set(all)].filter((v) => v.length >= minRedactedLength);
}

export class SecretRedactor {
  private readonly needles: Needle[];
  private readonly longest: number;
  // The needles' first characters: a held tail must start with one.
  private readonly starts: Set<string>;
  // Per text/reasoning block: the tail held back because it could be the start
  // of a value split across deltas.
  private readonly held = new Map<string, { type: string; tail: string }>();

  constructor(secrets: Map<string, string>) {
    const needles: Needle[] = [];
    for (const [name, value] of secrets) {
      if (value.length < minRedactedLength) continue;
      for (const text of variants(value)) {
        needles.push({ text, label: `[redacted:${name}]` });
      }
    }
    // Longest first so an encoding that contains another needle wins.
    needles.sort((a, b) => b.text.length - a.text.length);
    this.needles = needles;
    this.longest = needles.reduce((n, x) => Math.max(n, x.text.length), 0);
    this.starts = new Set(needles.map((x) => x.text[0]!));
  }

  get active(): boolean {
    return this.needles.length > 0;
  }

  redactString(input: string): string {
    let out = input;
    for (const needle of this.needles) {
      if (out.includes(needle.text))
        out = out.split(needle.text).join(needle.label);
    }
    return out;
  }

  // redactValue returns value itself when nothing in it changed, so a part
  // without a secret (most tool calls and outputs) is not copied.
  redactValue(value: unknown): unknown {
    if (typeof value === "string") return this.redactString(value);
    if (Array.isArray(value)) {
      const out = value.map((item) => this.redactValue(item));
      return out.some((item, i) => item !== value[i]) ? out : value;
    }
    if (value && typeof value === "object") {
      let out: Record<string, unknown> | undefined;
      for (const [key, item] of Object.entries(value)) {
        const redacted = this.redactValue(item);
        if (redacted !== item) {
          out ??= { ...(value as Record<string, unknown>) };
          out[key] = redacted;
        }
      }
      return out ?? value;
    }
    return value;
  }

  // apply returns the parts to publish for one emitted part: deltas may be
  // held back (a value can straddle two deltas) and released on the block's
  // end, so one input can yield zero, one or two parts.
  apply(part: Part): Part[] {
    if (!this.active) return [part];
    const type = String(part.type ?? "");
    const id = typeof part.id === "string" ? part.id : "";
    if ((type === "text-delta" || type === "reasoning-delta") && id) {
      const prior = this.held.get(id)?.tail ?? "";
      const combined = this.redactString(prior + String(part.delta ?? ""));
      const keep = this.holdBack(combined);
      const emit = combined.slice(0, combined.length - keep);
      this.held.set(id, { type, tail: combined.slice(combined.length - keep) });
      return emit ? [{ ...part, delta: emit }] : [];
    }
    if ((type === "text-end" || type === "reasoning-end") && id) {
      const release = this.release(id);
      return [...release, this.redactValue(part) as Part];
    }
    return [this.redactValue(part) as Part];
  }

  // flush releases every held tail (the stream ended without a block end).
  flush(): Part[] {
    const out: Part[] = [];
    for (const id of [...this.held.keys()]) out.push(...this.release(id));
    return out;
  }

  private release(id: string): Part[] {
    const held = this.held.get(id);
    this.held.delete(id);
    if (!held || !held.tail) return [];
    return [{ type: held.type, id, delta: held.tail }];
  }

  // holdBack is the length of the longest suffix of text that is a proper
  // prefix of some needle: those characters might complete a value next delta.
  private holdBack(text: string): number {
    const max = Math.min(text.length, this.longest - 1);
    for (let n = max; n > 0; n--) {
      if (!this.starts.has(text[text.length - n]!)) continue;
      const suffix = text.slice(text.length - n);
      if (
        this.needles.some((x) => x.text.length > n && x.text.startsWith(suffix))
      ) {
        return n;
      }
    }
    return 0;
  }
}
