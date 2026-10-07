import { describe, expect, it } from "vitest";
import {
  blueprintConfirmationFromError,
  blueprintTakeoverFromError,
  takeoverCopy,
} from "../takeover";
import en from "@/features/blueprints/locales/en";
import zh from "@/features/blueprints/locales/zh";
import { codedGraphQLError } from "@/test/mocks/apollo";

const PHRASE = "takeover blueprint blp-db136288mmqc73d4hpug";

// A minimal t that interpolates the real copy, so the assertions read the
// sentence a user sees.
const translator =
  (messages: typeof en) =>
  (key: string, params: Record<string, string> = {}) =>
    messages[key].message.replace(
      /\{(\w+)\}/g,
      (_, k: string) => params[k] ?? `{${k}}`,
    );
const t = translator(en) as never;
const tZh = translator(zh) as never;

describe("Blueprint takeover classification (w4/189)", () => {
  it("names the Blueprint a connection takeover replaces, from extensions", () => {
    const err = codedGraphQLError("BLUEPRINT_CONNECTION_CONFLICT", {
      blueprintId: "blp-db136288mmqc73d4hpug",
      blueprintName: "bex",
      repo: "https://github.com/bex-co/bex",
      branch: "main",
      path: "render.yaml",
      confirm: PHRASE,
    });
    const takeover = blueprintTakeoverFromError(err);
    expect(takeover).toMatchObject({
      kind: "connection",
      phrase: PHRASE,
      blueprintName: "bex",
    });
    const copy = takeoverCopy(takeover!, t);
    expect(copy.title).toBe("Replace Blueprint “bex”?");
    expect(copy.description).toContain(
      "render.yaml on https://github.com/bex-co/bex@main",
    );
    expect(copy.title).not.toMatch(/protected/i);
    expect(blueprintConfirmationFromError(err)).toEqual({
      status: "confirmation_required",
      confirmation: PHRASE,
      takeover,
    });
  });

  it("names the resource a resource takeover transfers", () => {
    const err = codedGraphQLError("BLUEPRINT_RESOURCE_CONFLICT", {
      resource: "docs",
      kind: "service",
      owningBlueprintId: "blp-db136288mmqc73d4hpug",
      confirm: PHRASE,
    });
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, t);
    expect(copy.title).toBe("Take over “docs”?");
    expect(copy.description).toBe(
      "Service “docs” is managed by Blueprint blp-db136288mmqc73d4hpug. Continuing transfers it to this Blueprint. Type the command below to continue.",
    );
  });

  // w5/m125: an unnamed owner arrives as server English ("another blueprint")
  // and the kind as the manifest kind (w5/098); the dialog translates both.
  it("renders a zh resource takeover with no English fragments", () => {
    const err = codedGraphQLError("BLUEPRINT_RESOURCE_CONFLICT", {
      resource: "cache",
      kind: "key_value",
      owningBlueprintId: "another blueprint",
      confirm: "takeover blueprint another blueprint",
    });
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, tZh);
    expect(copy.description).toBe(
      "Key Value “cache” 由另一个 Blueprint 管理。继续将把它转移到此 Blueprint。请输入下方命令以继续。",
    );
    expect(copy.description).not.toMatch(/another|key[ _]value/);
  });

  it.each([
    ["service", "服务"],
    ["postgres", "Postgres"],
    ["key_value", "Key Value"],
  ])("labels a %s takeover in zh", (kind, label) => {
    const err = codedGraphQLError("BLUEPRINT_RESOURCE_CONFLICT", {
      resource: "r",
      kind,
      owningBlueprintId: "blp-db136288mmqc73d4hpug",
      confirm: PHRASE,
    });
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, tZh);
    expect(copy.description.startsWith(`${label} “r”`)).toBe(true);
  });

  it("leaves a protected-environment refusal on the protected path", () => {
    // bex-api's own refusal (core.RequireProtectedConfirmation)
    // carries its phrase in extensions, like a takeover, under its own code.
    const err = codedGraphQLError(
      "PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED",
      {
        confirm: "sudo deploy service api",
        verb: "deploy",
        name: "api",
      },
    );
    expect(blueprintTakeoverFromError(err)).toBeNull();
    expect(blueprintConfirmationFromError(err)).toEqual({
      status: "confirmation_required",
      confirmation: "sudo deploy service api",
      resourceName: "api",
    });
  });
});
