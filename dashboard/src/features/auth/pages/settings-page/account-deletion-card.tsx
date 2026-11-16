import { useRef, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { Loader2, RefreshCw } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import {
  SudoCommandField,
  SudoPrompt,
} from "@/common/components/sudo-command-field";
import {
  billingBalanceDue,
  mutationErrorMessage,
  type BillingBalanceDue,
} from "@/common/lib/graphql-error";
import { BalanceDueAlert } from "@/common/components/balance-due-alert";
import { BillingSettlingAlert } from "@/common/components/billing-settling-alert";
import {
  isBillingSettling,
  useSettlingRetry,
} from "@/common/hooks/use-settling-retry";
import {
  clearBrowserAccountState,
  endBrowserSession,
} from "@/common/lib/ory/logout";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  AccountDeletionPreviewDocument,
  DeleteAccountDocument,
  type AccountWorkspaceDisposition,
} from "@/graphql/definitions";

export const accountDeletionConfirmation = "delete my account";

function WorkspaceList({
  rows,
  linkToSettings = false,
}: {
  rows: AccountWorkspaceDisposition[];
  linkToSettings?: boolean;
}) {
  const { t } = useTranslations();

  return (
    <ul className="list-disc space-y-1 pl-5 text-sm">
      {rows.map((workspace) => (
        <li key={workspace.id}>
          <span className="font-medium text-foreground">{workspace.name}</span>
          <span className="ml-2 font-mono text-xs text-muted-foreground">
            {workspace.id}
          </span>
          {linkToSettings ? (
            <a
              href={`/w/${encodeURIComponent(workspace.id)}/settings`}
              className="ml-2 font-medium text-primary underline-offset-4 hover:underline"
            >
              {t("auth.deleteAccountBlockedAction")}
            </a>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

/**
 * The preview read's loading frame (w4/686): the ready card's header, the
 * confirmation prompt, its field and the Delete button at ready geometry, the
 * copy reserved but not drawn, and one list row per workspace the account
 * belongs to. Which list each row joins, and a blocking workspace's alert, are
 * residuals.
 */
function PreviewSkeleton() {
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

export function AccountDeletionCard() {
  const { t } = useTranslations();
  const {
    data,
    loading,
    error: previewError,
    refetch,
  } = useQuery(AccountDeletionPreviewDocument);
  const [remove, { loading: deleting }] = useMutation(DeleteAccountDocument);
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [balanceDue, setBalanceDue] = useState<BillingBalanceDue | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submitGuard = useRef(false);
  const { run, settling } = useSettlingRetry();

  if (loading) return <PreviewSkeleton />;

  if (previewError || !data?.accountDeletionPreview) {
    return (
      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-destructive">
            {t("auth.deleteAccountTitle")}
          </CardTitle>
          <CardDescription>
            {t("auth.deleteAccountDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertTitle>{t("auth.deleteAccountPreviewErrorTitle")}</AlertTitle>
            <AlertDescription className="space-y-3">
              <p>{t("auth.deleteAccountPreviewError")}</p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void refetch()}
              >
                <RefreshCw />
                {t("auth.deleteAccountRetry")}
              </Button>
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }

  const preview = data.accountDeletionPreview;
  const blocked = preview.blocked.length > 0;
  const matches = confirmation === accountDeletionConfirmation;
  const busy = deleting || submitting;

  async function handleDelete() {
    if (!matches || blocked || busy || submitGuard.current) return;
    submitGuard.current = true;
    setSubmitting(true);
    setError(null);
    setBalanceDue(null);
    try {
      // BILLING_SETTLING (final usage still aggregating) retries every 15 s
      // for up to 3 minutes before it is shown as an error.
      await run(() => remove({ variables: { confirmation } }));
    } catch (cause) {
      submitGuard.current = false;
      setSubmitting(false);
      const due = billingBalanceDue(cause);
      if (due) setBalanceDue(due);
      else if (isBillingSettling(cause))
        setError(t("common.billingSettlingTimeout"));
      else setError(mutationErrorMessage(cause, t("auth.deleteAccountError")));
      return;
    }

    // Acceptance is durable, so local sign-out is best-effort: a temporary
    // Kratos logout failure must not turn a successful destructive request into
    // a misleading retry prompt. The worker independently revokes every
    // session. Clear process-local account data before a full-page transition
    // to the public, non-polling terminal state.
    try {
      await endBrowserSession();
    } catch {
      // Durable server-side deletion remains authoritative. Clear local state
      // even when Kratos is the dependency whose outage the worker is retrying.
      try {
        await clearBrowserAccountState();
      } catch {
        // The terminal full-page transition still drops this page's state.
      }
    }
    window.location.assign("/auth/account-deleted");
  }

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("auth.deleteAccountTitle")}
        </CardTitle>
        <CardDescription>{t("auth.deleteAccountDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {preview.delete.length > 0 ? (
          <div className="space-y-2">
            <h3 className="text-sm font-semibold">
              {t("auth.deleteAccountWillDelete")}
            </h3>
            <WorkspaceList rows={preview.delete} />
          </div>
        ) : null}
        {preview.leave.length > 0 ? (
          <div className="space-y-2">
            <h3 className="text-sm font-semibold">
              {t("auth.deleteAccountWillLeave")}
            </h3>
            <WorkspaceList rows={preview.leave} />
          </div>
        ) : null}
        {blocked ? (
          <Alert variant="destructive">
            <AlertTitle>{t("auth.deleteAccountBlockedTitle")}</AlertTitle>
            <AlertDescription className="space-y-2">
              <p>{t("auth.deleteAccountBlockedDescription")}</p>
              <WorkspaceList rows={preview.blocked} linkToSettings />
            </AlertDescription>
          </Alert>
        ) : null}

        <SudoCommandField
          id="delete-account-confirm"
          promptKey="auth.deleteAccountConfirmLabel"
          phrase={accountDeletionConfirmation}
          value={confirmation}
          onValueChange={setConfirmation}
          inputClassName="max-w-sm"
        />

        {settling ? <BillingSettlingAlert /> : null}

        {balanceDue ? (
          <BalanceDueAlert
            due={balanceDue}
            workspaceName={
              preview.delete.find(
                (workspace) => workspace.id === balanceDue.workspaceId,
              )?.name ?? balanceDue.workspaceId
            }
          />
        ) : null}

        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("auth.deleteAccountErrorTitle")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <Button
          variant="destructive"
          disabled={blocked || !matches || busy}
          onClick={() => void handleDelete()}
        >
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("auth.deleteAccountSubmit")}
        </Button>
      </CardContent>
    </Card>
  );
}
