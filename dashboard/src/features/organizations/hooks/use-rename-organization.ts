import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { RenameOrganizationDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { organizationErrorMessage } from "@/features/organizations/lib/organization-errors";

/**
 * An Owner renames the org (ADR115: `ChangeOrgNameEvent`). The mutation answers
 * the whole org, which the cache merges into the page and into the switcher's
 * entry for it.
 */
export function useRenameOrganization(organizationId: string) {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(RenameOrganizationDocument);

  const rename = useCallback(
    async (name: string) => {
      try {
        await mutate({ variables: { organizationId, name } });
        toast.success(t("organizations.nameSaved", { name }));
        return true;
      } catch (err) {
        toast.error(
          organizationErrorMessage(err, t, t("organizations.renameFailed")),
        );
        return false;
      }
    },
    [mutate, organizationId, t],
  );

  return { rename, saving: loading };
}
