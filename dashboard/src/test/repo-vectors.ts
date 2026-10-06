import { readFileSync } from "node:fs";

/**
 * A shared vector table from the Go side of the repo (a `testdata/*.json`
 * file both suites assert, so a rule cannot drift between bex-api and the
 * dashboard), read relative to the repo root. A row's `repeat` repeats its
 * string field that many times, for lengths no one should type into JSON.
 */
export function readRepoVectors<T>(path: string): T[] {
  return JSON.parse(readFileSync(`${process.cwd()}/../${path}`, "utf8"));
}

/** `value` repeated `repeat` times (once when unset). */
export function repeated(value: string, repeat?: number): string {
  return value.repeat(repeat ?? 1);
}
