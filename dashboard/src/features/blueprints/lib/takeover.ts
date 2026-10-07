import { CombinedGraphQLErrors } from "@apollo/client/errors";
import type { useTranslations } from "@/common/hooks/use-translations";
import { blueprintKindLabel } from "@/features/blueprints/lib/kind-labels";
import { protectedRefusalFromError } from "@/features/services/lib/protected-confirmation";
import type { BlueprintValidationError } from "@/features/blueprints/types";

/** bex-api's code for a repo+branch a live Blueprint already tracks (w4/m125). */
const BLUEPRINT_CONNECTION_CONFLICT = "BLUEPRINT_CONNECTION_CONFLICT";
/** bex-api's code for a resource another Blueprint manages (w8/m23). */
const BLUEPRINT_RESOURCE_CONFLICT = "BLUEPRINT_RESOURCE_CONFLICT";

/**
 * True when every validation problem is one an explicit takeover confirmation
 * resolves — a resource already owned by another blueprint (w8/m23), or a
 * repo+branch a live blueprint already tracks (w4/m125) — read from each
 * problem's code, never its wording (w5/m125).
 *
 * It is what keeps Deploy reachable in that case. Blocking Deploy on any
 * invalid preview is right for a manifest that does not parse — there is
 * nothing to confirm — but for a conflict it is a dead end: the phrase only
 * arrives in the create's refusal, so a disabled button means the user can
 * read about a takeover they can never perform.
 */
export function isTakeoverOnlyConflict(
  details: readonly Pick<BlueprintValidationError, "code">[],
): boolean {
  return (
    details.length > 0 &&
    details.every(
      ({ code }) =>
        code === BLUEPRINT_CONNECTION_CONFLICT ||
        code === BLUEPRINT_RESOURCE_CONFLICT,
    )
  );
}

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
    if (ext["code"] === BLUEPRINT_CONNECTION_CONFLICT) {
      return {
        kind: "connection",
        phrase,
        blueprintName: text("blueprintName") || text("blueprintId"),
        repo: text("repo"),
        branch: text("branch"),
        path: text("path"),
      };
    }
    if (ext["code"] === BLUEPRINT_RESOURCE_CONFLICT) {
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
  /** The protected resource the refusal names, for the dialog's copy. */
  resourceName?: string;
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
  const refusal = protectedRefusalFromError(err);
  return refusal
    ? {
        status: "confirmation_required",
        confirmation: refusal.confirm,
        resourceName: refusal.name,
      }
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
  const kindLabel = blueprintKindLabel(takeover.resourceKind);
  const kind = kindLabel ? t(kindLabel) : takeover.resourceKind;
  return {
    title: t("blueprints.takeoverResourceTitle", { name: takeover.resource }),
    // bex-api names the owner by id, or says "another blueprint" when the
    // claim has no readable owner: that is copy, not an id, so it is ours to
    // translate.
    description: takeover.owningBlueprintId.startsWith("blp-")
      ? t("blueprints.takeoverResourceBody", {
          kind,
          name: takeover.resource,
          owner: takeover.owningBlueprintId,
        })
      : t("blueprints.takeoverResourceBodyOtherOwner", {
          kind,
          name: takeover.resource,
        }),
  };
}
