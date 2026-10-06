import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { en } from "@/i18n";
import { statusReasonLabel } from "@/features/databases/lib/labels";

// w5/079: an unavailable database's statusReasonCode is the operator's Ready
// reason, and bex-api explains each one it knows with a sentence. Both lists
// are read from the Go source, so a reason added there without a translation
// here fails instead of quietly showing English to every other language.
const REPO_ROOT = `${process.cwd()}/..`;
const TYPES_GO = `${REPO_ROOT}/lego/types/v1alpha1/database_types.go`;
const POSTGRES_GO = `${REPO_ROOT}/lego/backend/internal/postgres/service.go`;

/** `ReasonFoo = "Foo"` → {ReasonFoo: "Foo"}. */
function reasonConstants(): Map<string, string> {
  const source = readFileSync(TYPES_GO, "utf8");
  return new Map(
    [...source.matchAll(/^\s*(Reason\w+)\s*=\s*"([^"]*)"/gm)].map(
      ([, name, value]) => [name, value],
    ),
  );
}

/** bex-api's unavailableReasons: reason constant → sentence. */
function explainedReasons(): Map<string, string> {
  const source = readFileSync(POSTGRES_GO, "utf8");
  const start = source.indexOf("var unavailableReasons = map[string]string{");
  const block = source.slice(start, source.indexOf("\n}\n", start));
  return new Map(
    [...block.matchAll(/appv1alpha1\.(Reason\w+):\s*"((?:[^"\\]|\\.)*)"/g)].map(
      ([, name, sentence]) => [name, sentence],
    ),
  );
}

describe("database status reason vocabulary", () => {
  it("translates every reason bex-api explains, with its sentence as the en copy", () => {
    const constants = reasonConstants();
    const explained = explainedReasons();
    expect(explained.size).toBeGreaterThan(8);
    for (const [name, sentence] of explained) {
      const code = constants.get(name);
      expect(code, `${name} is not a lego/types constant`).toBeDefined();
      const key = statusReasonLabel(code);
      expect(key, `no translation for ${code}`).toBeDefined();
      expect(en[key!], code).toBe(sentence);
    }
  });

  // bex-api passes the operator's own message through for this one, with the
  // sizes in it, so its copy is the dashboard's own.
  it("translates the storage shrink refusal", () => {
    expect(reasonConstants().get("ReasonStorageShrinkRejected")).toBe(
      "StorageShrinkRejected",
    );
    expect(statusReasonLabel("StorageShrinkRejected")).toBeDefined();
  });

  it("knows no code from an object's prototype", () => {
    expect(statusReasonLabel("toString")).toBeUndefined();
  });
});
