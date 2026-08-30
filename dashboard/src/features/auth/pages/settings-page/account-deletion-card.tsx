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
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import {
  clearBrowserAccountState,
  endBrowserSession,
} from "@/common/lib/ory/logout";
import { useTranslations } from "@/common/hooks/use-translations";
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

function PreviewSkeleton() {
  return (
    <Card
      aria-hidden="true"
      className="min-h-[388px] border-destructive/50 sm:min-h-[328px]"
    >
      <CardHeader className="gap-2">
        <Skeleton className="h-5 w-36" />
        <div className="space-y-2">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-11/12" />
          <Skeleton className="h-4 w-3/5 sm:hidden" />
        </div>
      </CardHeader>
      <CardContent className="space-y-5 sm:space-y-3">
        <div className="space-y-2">
          <Skeleton className="h-5 w-48" />
          <div className="flex items-start gap-2 pl-1">
            <Skeleton className="mt-1.5 size-2 shrink-0 rounded-full" />
            <div className="flex-1 space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2 sm:hidden" />
            </div>
          </div>
        </div>
        <div className="space-y-2">
          <Skeleton className="h-4 w-64 max-w-full" />
          <Skeleton className="h-3.5 w-28" />
          <Skeleton className="h-9 w-full max-w-sm" />
        </div>
        <Skeleton className="h-9 w-36" />
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
  const [submitting, setSubmitting] = useState(false);
  const submitGuard = useRef(false);

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
    try {
      await remove({ variables: { confirmation } });
    } catch (cause) {
      submitGuard.current = false;
      setSubmitting(false);
      setError(graphQLErrorMessage(cause) ?? t("auth.deleteAccountError"));
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
