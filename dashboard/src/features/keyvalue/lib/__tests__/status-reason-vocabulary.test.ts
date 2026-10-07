import { describe, expect, it } from "vitest";
import { en } from "@/i18n";
import { goReasonConstants, goReasonSentences } from "@/test/go-source";
import { statusReasonLabel } from "@/features/keyvalue/lib/labels";

// w5/m129: an unavailable Key Value's statusReasonCode is the operator's Ready
// reason, and bex-api explains each one it knows with a sentence. Both lists
// are read from the Go source, so a reason added there without a translation
// here fails instead of quietly showing English to every other language.
describe("Key Value status reason vocabulary", () => {
  it("translates every reason bex-api explains, with its sentence as the en copy", () => {
    const constants = goReasonConstants();
    const explained = goReasonSentences(
      "lego/backend/internal/keyvalue/status_reason.go",
      "unavailableReasons",
    );
    expect(explained.size).toBeGreaterThan(15);
    for (const [name, sentence] of explained) {
      const code = constants.get(name);
      expect(code, `${name} is not a lego/types constant`).toBeDefined();
      const key = statusReasonLabel(code);
      expect(key, `no translation for ${code}`).toBeDefined();
      expect(en[key!], code).toBe(sentence);
    }
  });

  // bex-api passes the operator's own message through for this one, so its
  // copy is the dashboard's own.
  it("translates the storage shrink refusal", () => {
    expect(goReasonConstants().get("ReasonStorageShrinkRejected")).toBe(
      "StorageShrinkRejected",
    );
    expect(statusReasonLabel("StorageShrinkRejected")).toBeDefined();
  });

  it("knows no code from an object's prototype", () => {
    expect(statusReasonLabel("toString")).toBeUndefined();
  });
});
