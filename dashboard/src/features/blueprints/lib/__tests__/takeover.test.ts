import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  blueprintConfirmationFromError,
  blueprintTakeoverFromError,
  takeoverCopy,
} from "../takeover";
import en from "@/features/blueprints/locales/en";
import zh from "@/features/blueprints/locales/zh";

const gqlError = (message: string, extensions: Record<string, unknown>) =>
  new CombinedGraphQLErrors({ errors: [{ message, extensions }] });

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
    const err = gqlError(
      `blueprint blp-x ("bex") already tracks … retry with confirm="${PHRASE}" to replace it`,
      {
        code: "BLUEPRINT_CONNECTION_CONFLICT",
        blueprintId: "blp-db136288mmqc73d4hpug",
        blueprintName: "bex",
        repo: "https://github.com/bex-co/bex",
        branch: "main",
        path: "render.yaml",
        confirm: PHRASE,
      },
    );
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
    const err = gqlError(
      `service "docs" is managed by blueprint blp-db136288mmqc73d4hpug; retry with confirm="${PHRASE}" to transfer ownership to this blueprint`,
      {
        code: "BLUEPRINT_RESOURCE_CONFLICT",
        resource: "docs",
        kind: "service",
        owningBlueprintId: "blp-db136288mmqc73d4hpug",
        confirm: PHRASE,
      },
    );
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, t);
    expect(copy.title).toBe("Take over “docs”?");
    expect(copy.description).toBe(
      "Service “docs” is managed by Blueprint blp-db136288mmqc73d4hpug. Continuing transfers it to this Blueprint. Type the command below to continue.",
    );
  });

  // w5/m125: the kind and an unnamed owner arrive as server English ("key
  // value", "another blueprint"); the dialog translates both.
  it("renders a zh resource takeover with no English fragments", () => {
    const err = gqlError(
      `key value "cache" is managed by blueprint another blueprint; retry with confirm="takeover blueprint another blueprint" to transfer ownership to this blueprint`,
      {
        code: "BLUEPRINT_RESOURCE_CONFLICT",
        resource: "cache",
        kind: "key value",
        owningBlueprintId: "another blueprint",
        confirm: "takeover blueprint another blueprint",
      },
    );
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, tZh);
    expect(copy.description).toBe(
      "Key Value “cache” 由另一个 Blueprint 管理。继续将把它转移到此 Blueprint。请输入下方命令以继续。",
    );
    expect(copy.description).not.toMatch(/another|key value/);
  });

  it.each([
    ["service", "服务"],
    ["database", "Postgres"],
    ["key_value", "Key Value"],
  ])("labels a %s takeover in zh", (kind, label) => {
    const err = gqlError("managed by blueprint", {
      code: "BLUEPRINT_RESOURCE_CONFLICT",
      resource: "r",
      kind,
      owningBlueprintId: "blp-db136288mmqc73d4hpug",
      confirm: PHRASE,
    });
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, tZh);
    expect(copy.description.startsWith(`${label} “r”`)).toBe(true);
  });

  it("leaves a protected-environment refusal on the protected path", () => {
    // bex-api's own refusal (lego/backend/internal/apps/protection.go): a bad
    // request with no code, naming the phrase only in its text.
    const err = gqlError(
      'bad request: "api" is a member of a protected environment; retry with confirm="sudo deploy service api" to deploy it',
      {},
    );
    expect(blueprintTakeoverFromError(err)).toBeNull();
    expect(blueprintConfirmationFromError(err)).toEqual({
      status: "confirmation_required",
      confirmation: "sudo deploy service api",
    });
  });
});
