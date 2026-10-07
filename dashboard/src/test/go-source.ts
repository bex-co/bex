import { readFileSync, readdirSync } from "node:fs";

// The Go side of the repo, read by tests that keep a dashboard vocabulary in
// step with bex-api's (paths relative to the repo root).
const REPO_ROOT = `${process.cwd()}/..`;
const TYPES_DIR = `${REPO_ROOT}/lego/types/v1alpha1`;

/** Every `ReasonFoo = "Foo"` constant in lego/types/v1alpha1, by name. */
export function goReasonConstants(): Map<string, string> {
  return new Map(
    readdirSync(TYPES_DIR)
      .filter((file) => file.endsWith(".go") && !file.endsWith("_test.go"))
      .flatMap((file) => [
        ...readFileSync(`${TYPES_DIR}/${file}`, "utf8").matchAll(
          /^\s*(Reason\w+)\s*=\s*"([^"]*)"/gm,
        ),
      ])
      .map(([, name, value]) => [name, value]),
  );
}

/**
 * A Go `var <name> = map[string]string{appv1alpha1.ReasonFoo: "…"}` in the
 * file at path: reason constant name → sentence.
 */
export function goReasonSentences(
  path: string,
  name: string,
): Map<string, string> {
  const source = readFileSync(`${REPO_ROOT}/${path}`, "utf8");
  const start = source.indexOf(`var ${name} = map[string]string{`);
  if (start < 0) throw new Error(`${path} declares no ${name}`);
  const block = source.slice(start, source.indexOf("\n}\n", start));
  return new Map(
    [...block.matchAll(/appv1alpha1\.(Reason\w+):\s*"((?:[^"\\]|\\.)*)"/g)].map(
      ([, constant, sentence]) => [constant, sentence],
    ),
  );
}
