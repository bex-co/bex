import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetEnvironmentAclDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface EnvironmentACLInput {
  protectedStatus: "protected" | "unprotected";
  networkIsolationEnabled: boolean;
  ipAllowList: string[];
}

export interface UseSetEnvironmentACLResult {
  saveACL: (
    id: string,
    environmentName: string,
    acl: EnvironmentACLInput,
  ) => Promise<boolean>;
  saving: boolean;
}

/** Full-replaces the three fields in an Environment ACL in one mutation. */
export function useSetEnvironmentACL(): UseSetEnvironmentACLResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetEnvironmentAclDocument, {
    refetchQueries: ["Environments"],
    awaitRefetchQueries: true,
  });
  const [saving, setSaving] = useState(false);

  const saveACL = useCallback(
    async (id: string, environmentName: string, acl: EnvironmentACLInput) => {
      setSaving(true);
      try {
        await mutate({ variables: { id, ...acl } });
        toast.success(
          t("environments.aclSaveSuccess", { name: environmentName }),
        );
        return true;
      } catch {
        toast.error(t("environments.aclSaveError", { name: environmentName }));
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutate, t],
  );

  return { saveACL, saving };
}
