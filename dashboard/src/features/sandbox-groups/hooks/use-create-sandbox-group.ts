import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CreateSandboxGroupDocument,
  type CreateSandboxGroupMutation,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  graphQLErrorExtensions,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/** The only region a sandbox group runs in (Render has one; bex mirrors it). */
export const SANDBOX_GROUP_REGION = "oregon";

/** The longest group name bex-api accepts (`maxGroupName`, in characters). */
export const SANDBOX_GROUP_NAME_MAX = 100;

/** The error codes bex-api answers a refused group create with. */
const NAME_TAKEN = "SANDBOX_GROUP_NAME_TAKEN";
const NAME_INVALID = "SANDBOX_GROUP_NAME_INVALID";
/** Render's own `errorCode`: the workspace is at its sandbox group limit. */
const LIMIT_REACHED = "sandbox_group_limit_reached";

export type SandboxGroup = NonNullable<
  CreateSandboxGroupMutation["createSandboxGroup"]
>;

/**
 * Why the last create was refused, for inline display: a name refusal sits
 * beside the name field, the group-limit refusal is a form-level alert.
 */
export interface SandboxGroupRefusal {
  kind: "name" | "limit";
  message: string;
}

export interface UseCreateSandboxGroupResult {
  /** Fires createSandboxGroup; resolves the new group, or null on failure. */
  create: (name: string) => Promise<SandboxGroup | null>;
  busy: boolean;
  /** The last refusal worth showing inline; null once cleared or on success. */
  refusal: SandboxGroupRefusal | null;
  /** Clear `refusal` (on input change and when the dialog closes). */
  clearRefusal: () => void;
}

/**
 * Wires the "New Sandbox Group" dialog to bex-api's `createSandboxGroup`
 * (ADR104, w22/m311). Scoped to the switcher's selected workspace, as every
 * other create hook is.
 *
 * A duplicate name and the workspace's group limit are the two refusals a user
 * can act on, so they are keyed on bex-api's error code (never its English
 * prose), localized here and shown inline. Every other failure relays the
 * server's own reason in a toast, or the generic copy for a transport failure.
 */
export function useCreateSandboxGroup(): UseCreateSandboxGroupResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateSandboxGroupDocument);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<SandboxGroupRefusal | null>(null);
  const clearRefusal = useCallback(() => setRefusal(null), []);

  const create = useCallback(
    async (name: string) => {
      // Never sent with a null ownerId, which bex-api would silently route to
      // the caller's default workspace (mirrors useCreateApiKey).
      if (currentWorkspaceId == null) {
        toast.error(t("sandboxGroups.createError", { name }));
        return null;
      }
      setBusy(true);
      setRefusal(null);
      try {
        const res = await mutate({
          variables: {
            name,
            region: SANDBOX_GROUP_REGION,
            ownerId: currentWorkspaceId,
          },
        });
        const group = res.data?.createSandboxGroup;
        if (!group?.id) throw new Error("createSandboxGroup returned no id");
        return group;
      } catch (err) {
        const inline = inlineRefusal(err, name, t);
        if (inline) setRefusal(inline);
        else
          toast.error(
            mutationErrorMessage(err, t("sandboxGroups.createError", { name })),
          );
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { create, busy, refusal, clearRefusal };
}

/** The localized inline refusal for a coded error, or null for any other. */
function inlineRefusal(
  err: unknown,
  name: string,
  t: ReturnType<typeof useTranslations>["t"],
): SandboxGroupRefusal | null {
  if (graphQLErrorExtensions(err, NAME_TAKEN)) {
    return {
      kind: "name",
      message: t("sandboxGroups.errorNameTaken", { name }),
    };
  }
  const invalid = graphQLErrorExtensions(err, NAME_INVALID);
  if (invalid) {
    const max = invalid["maxLength"];
    return {
      kind: "name",
      message: t("sandboxGroups.errorNameInvalid", {
        max: typeof max === "number" ? max : SANDBOX_GROUP_NAME_MAX,
      }),
    };
  }
  const limit = graphQLErrorExtensions(err, LIMIT_REACHED);
  if (limit) {
    const count = limit["limit"];
    return {
      kind: "limit",
      message:
        typeof count === "number"
          ? t("sandboxGroups.errorLimitReached", { count })
          : t("sandboxGroups.errorLimitReachedGeneric"),
    };
  }
  return null;
}
