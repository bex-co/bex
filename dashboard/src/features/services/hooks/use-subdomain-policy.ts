import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetSubdomainPolicyDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseSubdomainPolicyResult {
  setSubdomainPolicy: (id: string, policy: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Custom Domains platform-subdomain toggle to bex-api's
 * `setSubdomainPolicy` (w7/m31) — Render's renderSubdomainPolicy field
 * (enabled | disabled).
 */
export function useSubdomainPolicy(): UseSubdomainPolicyResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetSubdomainPolicyDocument);
  const [busy, setBusy] = useState(false);

  const setSubdomainPolicy = useCallback(
    async (id: string, policy: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, policy } });
        toast.success(t("services.subdomainPolicySuccess"));
        return true;
      } catch {
        toast.error(t("services.subdomainPolicyError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setSubdomainPolicy, busy };
}
