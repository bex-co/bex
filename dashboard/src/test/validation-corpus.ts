import { readdirSync } from "node:fs";
import { readRepoVectors, repeated } from "./repo-vectors";

// The validation corpus: one JSON file per validator family under
// lego/backend/testdata/validation, asserted by a Go test against the server's
// validator (lego/backend/internal/validationcorpus) and by
// `__tests__/validation-corpus.test.ts` against the client validator that
// mirrors it, so a form and bex-api cannot disagree on an input without a test
// failing.

export const CORPUS_DIR = "lego/backend/testdata/validation";

export interface CorpusCase {
  /** The value. `repeat` repeats `input`, and `prefix` and `suffix` go around it. */
  prefix?: string;
  input: string;
  repeat?: number;
  suffix?: string;
  /** The variable name of the env-value family. */
  key?: string;
  /** The server's verdict. */
  valid: boolean;
  /** A refused input the client may still accept: only the server can judge it. */
  serverOnly?: boolean;
  why?: string;
}

/** The families with a corpus file, by name. */
export function corpusFamilies(): string[] {
  return readdirSync(`${process.cwd()}/../${CORPUS_DIR}`)
    .filter((file) => file.endsWith(".json"))
    .map((file) => file.replace(/\.json$/, ""))
    .sort();
}

export function readCorpus(family: string): CorpusCase[] {
  return readRepoVectors<CorpusCase>(`${CORPUS_DIR}/${family}.json`);
}

/** The case's text: its input repeated, with its prefix and suffix. */
export function corpusText(c: CorpusCase): string {
  return (c.prefix ?? "") + repeated(c.input, c.repeat) + (c.suffix ?? "");
}

/** The case for a failure message, cut to a readable length. */
function caseLabel(c: CorpusCase): string {
  const text = corpusText(c);
  return text.length <= 80
    ? JSON.stringify(text)
    : `${JSON.stringify(text.slice(0, 40))}… (${text.length} characters)`;
}

/**
 * Every case where the client disagrees with the server: a client that refuses
 * what the server accepts blocks a valid submit, and one that accepts what the
 * server refuses is right only on a `serverOnly` case, where just the server
 * can judge. Empty when the client and the corpus agree.
 */
export function corpusMismatches(
  cases: CorpusCase[],
  accepts: (text: string, c: CorpusCase) => boolean,
): string[] {
  return cases.flatMap((c) => {
    const got = accepts(corpusText(c), c);
    if (got === c.valid) return [];
    if (got && c.serverOnly) return [];
    return [
      `${caseLabel(c)}: the client ${got ? "accepts" : "refuses"} it, the server ${
        c.valid ? "accepts" : "refuses"
      }${c.why ? ` (${c.why})` : ""}`,
    ];
  });
}
