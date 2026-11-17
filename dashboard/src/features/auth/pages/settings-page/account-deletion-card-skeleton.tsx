import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { SudoPrompt } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { accountDeletionConfirmation } from "@/features/auth/lib/account-deletion";

/**
 * The preview read's loading frame (w4/686): the ready card's header, the
 * confirmation prompt, its field and the Delete button at ready geometry, the
 * copy reserved but not drawn, and one list row per workspace the account
 * belongs to. Which list each row joins, and a blocking workspace's alert, are
 * residuals.
 */
export function AccountDeletionCardSkeleton() {
  const { t } = useTranslations();
  // Each of the account's workspaces is one row of the preview (deleted, left
  // or blocking), listed by id as the preview lists them. Which list a row
  // lands in depends on the workspace's members, so the first is drawn under
  // "will delete" and the rest under "will leave". Each row reserves its own
  // name and id, so a row that wraps at this width wraps here too.
  const { workspaces } = useWorkspace();
  const rows = [...workspaces].sort((a, b) =>
    a.id < b.id ? -1 : a.id > b.id ? 1 : 0,
  );
  const lists = rows.length > 1 ? [rows.slice(0, 1), rows.slice(1)] : [rows];
  return (
    <Card
      aria-hidden="true"
      className="border-destructive/50"
      data-testid="account-deletion-skeleton"
    >
      <CardHeader>
        <div className="relative w-fit leading-none font-semibold">
          <ReservedText>{t("auth.deleteAccountTitle")}</ReservedText>
        </div>
        <div className="relative text-sm">
          <ReservedText>{t("auth.deleteAccountDescription")}</ReservedText>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {lists.map((listed, list) => (
          <div key={list} className="space-y-2">
            <Skeleton className="h-5 w-48" />
            <ul className="list-disc space-y-1 pl-5 text-sm">
              {listed.length === 0 ? (
                <li data-testid="account-deletion-row-placeholder">
                  <Skeleton className="inline-block h-4 w-3/4 align-middle" />
                </li>
              ) : (
                listed.map((workspace) => (
                  <li
                    key={workspace.id}
                    className="relative"
                    data-testid="account-deletion-row-placeholder"
                  >
                    <ReservedText>
                      <span className="font-medium">{workspace.name}</span>
                      <span className="ml-2 font-mono text-xs">
                        {workspace.id}
                      </span>
                    </ReservedText>
                  </li>
                ))
              )}
            </ul>
          </div>
        ))}
        <div className="space-y-2">
          <p className="relative text-sm">
            <ReservedText>
              <SudoPrompt
                promptKey="auth.deleteAccountConfirmLabel"
                phrase={accountDeletionConfirmation}
              />
            </ReservedText>
          </p>
          <div className="relative flex w-fit text-sm leading-none font-medium">
            <ReservedText>{t("common.sudoCommandLabel")}</ReservedText>
          </div>
          <Skeleton className="h-9 w-full max-w-sm" />
        </div>
        <ReservedButton variant="destructive">
          {t("auth.deleteAccountSubmit")}
        </ReservedButton>
      </CardContent>
    </Card>
  );
}
