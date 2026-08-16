import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateServiceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

export interface EnvVarEntry {
  key: string;
  value: string;
}

export interface SecretFileEntry {
  name: string;
  content: string;
}

export interface CreateServiceInput {
  name: string;
  type?: string;
  environmentId?: string;
  repo?: string;
  image?: string;
  registryCredentialId?: string;
  branch?: string;
  rootDir?: string;
  runtime?:
    | "docker"
    | "elixir"
    | "go"
    | "image"
    | "node"
    | "python"
    | "ruby"
    | "rust";
  buildCommand?: string;
  startCommand?: string;
  dockerfilePath?: string;
  /** Render's Build Filters (glob paths/ignoredPaths) gating git-push auto-deploys. */
  buildFilter?: { paths: string[]; ignoredPaths: string[] };
  plan?: string;
  autoDeploy?: boolean;
  schedule?: string;
  command?: string;
  publishPath?: string;
  envVars?: EnvVarEntry[];
  secretFiles?: SecretFileEntry[];
}

export interface CreateServiceResult {
  id: string;
  /** First deploy id — only present for git-sourced services (w3/m14). */
  deployId?: string;
}

export interface UseCreateServiceResult {
  create: (input: CreateServiceInput) => Promise<CreateServiceResult | null>;
  busy: boolean;
  /**
   * The workspace's service cap was hit. Carries the backend's exact message
   * so the create form can show it inline with an upgrade CTA (w7/m9). Null
   * when the last attempt failed for another reason (toasted) or hasn't failed.
   */
  capLimit: string | null;
  /**
   * The name was already in use in the target workspace (w4/m19) — the race
   * window between the debounced availability check and submit (someone else
   * claimed it in between, or the check hadn't settled yet). Shown inline,
   * same shape as the live check, rather than a generic toast. Null when the
   * last attempt failed for another reason (toasted) or hasn't failed.
   */
  nameConflict: boolean;
  /** Clears nameConflict — call when the user edits the name away from it. */
  clearNameConflict: () => void;
}

/**
 * Wires the create wizard to bex-api's `createService`, scoped to the switcher's
 * selected workspace (w6/m14): `ownerId` names the workspace the service is
 * created in, mirroring `useServices`'s list read. As there the selection is the
 * gate — a create is refused (never sent with a null ownerId, which the backend
 * would silently route to the caller's default workspace) until the workspace
 * list resolves; that's the list hooks' `skip` in mutation form.
 */
export function useCreateService(): UseCreateServiceResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate, { loading: busy }] = useMutation(CreateServiceDocument);
  const [capLimit, setCapLimit] = useState<string | null>(null);
  const [nameConflict, setNameConflict] = useState(false);
  const paymentGate = usePaymentRequiredGate();

  const create = useCallback(
    async (input: CreateServiceInput) => {
      if (currentWorkspaceId == null) {
        toast.error(t("services.createError", { name: input.name }));
        return null;
      }
      setCapLimit(null);
      setNameConflict(false);
      try {
        const res = await paymentGate.run(() =>
          mutate({
            variables: {
              ...input,
              ownerId: currentWorkspaceId,
              // An empty list is "no rows staged", not "clear them" — the
              // create editors' blank placeholder row must not reach the wire.
              envVars: input.envVars?.length ? input.envVars : undefined,
              secretFiles: input.secretFiles?.length
                ? input.secretFiles
                : undefined,
            },
          }),
        );
        const svc = res.data?.createService;
        const id = svc?.id;
        if (!id) throw new Error("createService returned no id");
        const deployId = svc?.latestDeployId ?? undefined;
        toast.success(t("services.createSuccess", { name: input.name }));
        return { id, deployId };
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return null;
        // "workspace is limited to N services" — surface inline with upgrade CTA
        // rather than a toast that leaves the user with nowhere to go (w7/m9).
        // "name ... is already in use" (w4/m19) — a raced duplicate the
        // debounced check missed; same inline treatment, not a toast.
        const msg = graphQLErrorMessage(err) ?? "";
        if (msg.toLowerCase().includes("workspace is limited")) {
          setCapLimit(msg);
        } else if (msg.toLowerCase().includes("already in use")) {
          setNameConflict(true);
        } else {
          toast.error(t("services.createError", { name: input.name }));
        }
        return null;
      }
    },
    [mutate, paymentGate, t, currentWorkspaceId],
  );

  const clearNameConflict = useCallback(() => setNameConflict(false), []);

  return { create, busy, capLimit, nameConflict, clearNameConflict };
}
