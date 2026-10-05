import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  blueprintConfirmationFromError,
  blueprintTakeoverFromError,
  takeoverCopy,
} from "../takeover";
import en from "@/features/blueprints/locales/en";

const gqlError = (message: string, extensions: Record<string, unknown>) =>
  new CombinedGraphQLErrors({ errors: [{ message, extensions }] });

const PHRASE = "takeover blueprint blp-db136288mmqc73d4hpug";

// A minimal t that interpolates the real en copy, so the assertions read the
// sentence a user sees.
const t = ((key: string, params: Record<string, string> = {}) =>
  en[key].message.replace(
    /\{(\w+)\}/g,
    (_, k: string) => params[k] ?? `{${k}}`,
  )) as never;

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
      `static_site "docs" is managed by blueprint blp-old; retry with confirm="${PHRASE}"`,
      {
        code: "BLUEPRINT_RESOURCE_CONFLICT",
        resource: "docs",
        kind: "static_site",
        owningBlueprintId: "blp-old",
        confirm: PHRASE,
      },
    );
    const copy = takeoverCopy(blueprintTakeoverFromError(err)!, t);
    expect(copy.title).toBe("Take over “docs”?");
    expect(copy.description).toContain("managed by Blueprint blp-old");
  });

  it("leaves a protected-environment refusal on the protected path", () => {
    const err = gqlError(
      'service api belongs to a protected environment; retry with confirm="sudo deploy service api"',
      { code: "PROTECTED_ENVIRONMENT" },
    );
    expect(blueprintTakeoverFromError(err)).toBeNull();
    expect(blueprintConfirmationFromError(err)).toEqual({
      status: "confirmation_required",
      confirmation: "sudo deploy service api",
    });
  });
});
