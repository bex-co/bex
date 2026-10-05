import { CombinedGraphQLErrors } from "@apollo/client/errors";
import type { useTranslations } from "@/common/hooks/use-translations";
import { protectedConfirmationFromError } from "@/features/services/lib/protected-confirmation";

/**
 * A Blueprint takeover bex-api refused until confirmed, classified from the
 * error's extensions.code — never its message — so the dialog can name what is
 * being replaced instead of borrowing the protected-environment copy (w4/189).
 * The phrase is still the server's, verbatim: it is the authorization.
 */
export type BlueprintTakeover =
  | {
      kind: "connection";
      phrase: string;
      blueprintName: string;
      repo: string;
      branch: string;
      path: string;
    }
  | {
      kind: "resource";
      phrase: string;
      resource: string;
      resourceKind: string;
      owningBlueprintId: string;
    };

export function blueprintTakeoverFromError(
  err: unknown,
): BlueprintTakeover | null {
  if (!CombinedGraphQLErrors.is(err)) return null;
  for (const item of err.errors) {
    const ext = item.extensions ?? {};
    const phrase = typeof ext["confirm"] === "string" ? ext["confirm"] : "";
    if (!phrase) continue;
    const text = (key: string) =>
      typeof ext[key] === "string" ? (ext[key] as string) : "";
    if (ext["code"] === "BLUEPRINT_CONNECTION_CONFLICT") {
      return {
        kind: "connection",
        phrase,
        blueprintName: text("blueprintName") || text("blueprintId"),
        repo: text("repo"),
        branch: text("branch"),
        path: text("path"),
      };
    }
    if (ext["code"] === "BLUEPRINT_RESOURCE_CONFLICT") {
      return {
        kind: "resource",
        phrase,
        resource: text("resource"),
        resourceKind: text("kind"),
        owningBlueprintId: text("owningBlueprintId"),
      };
    }
  }
  return null;
}

/** A Blueprint mutation refused until the user types the server's phrase. */
export interface BlueprintConfirmationRequired {
  status: "confirmation_required";
  confirmation: string;
  /** Set when the refusal is a Blueprint takeover, not a protected env. */
  takeover?: BlueprintTakeover;
}

/** The typed-confirmation retry a refused Blueprint create or sync asks for. */
export function blueprintConfirmationFromError(
  err: unknown,
): BlueprintConfirmationRequired | null {
  const takeover = blueprintTakeoverFromError(err);
  if (takeover) {
    return {
      status: "confirmation_required",
      confirmation: takeover.phrase,
      takeover,
    };
  }
  const phrase = protectedConfirmationFromError(err);
  return phrase
    ? { status: "confirmation_required", confirmation: phrase }
    : null;
}

/** The takeover dialog's title and body, from the server's own details. */
export function takeoverCopy(
  takeover: BlueprintTakeover,
  t: ReturnType<typeof useTranslations>["t"],
): { title: string; description: string } {
  if (takeover.kind === "connection") {
    return {
      title: t("blueprints.takeoverConnectionTitle", {
        name: takeover.blueprintName,
      }),
      description: t("blueprints.takeoverConnectionBody", {
        name: takeover.blueprintName,
        path: takeover.path,
        repo: takeover.repo,
        branch: takeover.branch,
      }),
    };
  }
  return {
    title: t("blueprints.takeoverResourceTitle", { name: takeover.resource }),
    description: t("blueprints.takeoverResourceBody", {
      kind: takeover.resourceKind,
      name: takeover.resource,
      owner: takeover.owningBlueprintId,
    }),
  };
}
