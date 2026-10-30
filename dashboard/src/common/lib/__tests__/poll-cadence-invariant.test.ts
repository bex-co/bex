import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import {
  QUIC_IDLE_TIMEOUT_MS,
  RESOURCE_POLL_INTERVAL_MS,
} from "@/common/lib/polling";

/**
 * A recurring request near Chrome's QUIC idle timeout races the session close
 * and fails with ERR_CONNECTION_CLOSED (w3/m244: a 30s poll lost ~1 in 9
 * requests). Every numeric literal in the danger band must be a recurring
 * request deliberately moved out of it, or listed here with the reason it is
 * not a recurring request.
 */
const SRC_DIR = join(import.meta.dirname, "../../..");
const BAND_MS = 3_000;
const LITERAL = /\b\d[\d_]*\b/g;

const ALLOWED: Record<string, string> = {
  "features/capabilities/lib/capability-policy.ts: 30_000":
    "a snapshot freshness bound that outlives one baseline poll, not a request",
  "features/router/page.tsx: 30_000": "a local clock tick, no network",
  "common/lib/polling.ts: 29_000": "the timeout itself",
};

function readSources(): Map<string, string> {
  const sources = new Map<string, string>();
  for (const path of readdirSync(SRC_DIR, {
    recursive: true,
    encoding: "utf8",
  })) {
    if (
      /\.tsx?$/.test(path) &&
      !/\.test\.tsx?$/.test(path) &&
      !path.split(/[\\/]/).includes("__tests__")
    ) {
      sources.set(path, readFileSync(join(SRC_DIR, path), "utf8"));
    }
  }
  return sources;
}

describe("poll cadence vs the HTTP/3 idle timeout (w3/m244)", () => {
  it("keeps the baseline poll below the danger band", () => {
    expect(RESOURCE_POLL_INTERVAL_MS).toBeLessThan(
      QUIC_IDLE_TIMEOUT_MS - BAND_MS,
    );
  });

  it("has no unexplained interval inside the danger band", () => {
    const sources = readSources();
    expect(sources.size).toBeGreaterThan(300);
    const offenders: string[] = [];
    for (const [path, text] of sources) {
      for (const [literal] of text.matchAll(LITERAL)) {
        const ms = Number(literal.replaceAll("_", ""));
        if (Math.abs(ms - QUIC_IDLE_TIMEOUT_MS) > BAND_MS) continue;
        const key = `${path}: ${literal}`;
        if (!(key in ALLOWED)) offenders.push(key);
      }
    }
    expect(offenders).toEqual([]);
  });
});
