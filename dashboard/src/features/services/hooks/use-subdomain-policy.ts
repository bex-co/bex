import { SetSubdomainPolicyDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

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
  const { run, busy } = useFieldMutation(
    SetSubdomainPolicyDocument,
    (id: string, policy: string) => ({ id, policy }),
    {
      success: "services.subdomainPolicySuccess",
      error: "services.subdomainPolicyError",
    },
  );

  return { setSubdomainPolicy: run, busy };
}
