import { useCallback, useState } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { organizationErrorMessage } from "@/features/organizations/lib/organization-errors";

/**
 * A table's org actions, one row at a time: `busy` is the key (a subject, an
 * invite or a workspace id) whose action is in flight. Success is a toast;
 * a refusal is a toast in the user's words (`organizationErrorMessage`, which
 * names an ORGANIZATION_MEMBER_LAST_ADMIN workspace through `workspaceName`).
 */
export function useKeyedOrganizationAction(
  workspaceName?: (id: string) => string | undefined,
) {
  const { t } = useTranslations();
  const [busy, setBusy] = useState<string | null>(null);

  const run = useCallback(
    async (
      key: string,
      action: () => Promise<unknown>,
      success: string,
      fallback: string,
    ) => {
      setBusy(key);
      try {
        await action();
        toast.success(success);
        return true;
      } catch (err) {
        toast.error(organizationErrorMessage(err, t, fallback, workspaceName));
        return false;
      } finally {
        setBusy(null);
      }
    },
    [t, workspaceName],
  );

  return { busy, run };
}

/**
 * A dialog's org action: its refusal stays in the dialog, in the user's words,
 * until the user edits the form (`clearRefusal`) or tries again; success is a
 * toast with the sentence `action` resolves to.
 */
export function useOrganizationDialogAction() {
  const { t } = useTranslations();
  const [refusal, setRefusal] = useState<string | null>(null);

  const run = useCallback(
    async (action: () => Promise<string>, fallback: string) => {
      setRefusal(null);
      try {
        toast.success(await action());
        return true;
      } catch (err) {
        setRefusal(organizationErrorMessage(err, t, fallback));
        return false;
      }
    },
    [t],
  );

  return {
    run,
    refusal,
    clearRefusal: useCallback(() => setRefusal(null), []),
  };
}
