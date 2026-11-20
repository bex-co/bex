import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { PlugZap, Plus, ShieldAlert, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { GithubIcon } from "@/common/components/github-icon";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { PanelCenteredState } from "@/common/components/panel-states";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useGitConnections,
  type GitConnectionRow,
} from "@/features/git/hooks/use-git-connection";
import {
  useConnectGit,
  useConnectGitAccount,
} from "@/features/git/hooks/use-connect-git";
import {
  useClaimSelection,
  type ClaimCandidate,
  type UseClaimSelectionResult,
} from "@/features/git/hooks/use-claim-selection";
import { useDisconnectGit } from "@/features/git/hooks/use-disconnect-git";
import { useForgetCachedRepos } from "@/features/git/hooks/use-forget-cached-repos";
import { isGitHubUnavailable } from "@/features/git/lib/errors";
import { GitConnectionsSkeleton } from "@/features/git/components/connect-github-card-skeleton";

// An org member's install request (setup_action=request) is waiting on the
// org's owners, not failed: connecting again now cannot include that org, so
// its alert offers no restart action and is not styled as an error.
const INSTALL_REQUESTED = "install_requested";

// The bounded git_error codes the callback redirects with (backend
// internal/github/rest.go). missing_state is the direct-github.com-install case;
// GitHub strips the state for already-installed accounts (ADR078 §3a), so every
// recovery is the single "Connect GitHub account" action (§3b), which starts
// with OAuth and lands on the account chooser — never the install URL.
//
// w2/m162: no message here may tell a user to install an App they have installed
// or to uninstall one. Under N:N (ADR078 §2) an account bound to another
// workspace is claimable, and ambiguity is resolved by the chooser below, so both
// of those old instructions described states that no longer exist.
function callbackErrorMessage(t: (k: string) => string, code: string): string {
  switch (code) {
    case INSTALL_REQUESTED:
      return t("git.callbackErrorInstallRequested");
    case "expired_state":
      return t("git.callbackErrorExpired");
    case "missing_state":
      return t("git.callbackErrorMissing");
    case "invalid_state":
      return t("git.callbackErrorInvalid");
    case "no_claimable_installation":
      return t("git.callbackErrorNoClaimable");
    case "ambiguous_installation":
      return t("git.callbackErrorAmbiguous");
    default:
      return t("git.callbackErrorGeneric");
  }
}

/**
 * Settings → Connect GitHub (w2/m8, multi-account ADR078): lists every GitHub
 * account/org the workspace has connected — each with a Manage-access link and a
 * per-account Disconnect — plus ONE connect action (ADR078 §3b, w1/m526) that
 * starts with GitHub OAuth and returns here to the account chooser. Also renders
 * the unavailable (no GitHub App) and error states, and a callback alert.
 * Returning from GitHub's callback lands on /settings#integrations, so the card
 * refetches on mount and on window focus.
 */
export function ConnectGithubCard({
  callbackError,
  claimSelectionId,
}: {
  callbackError?: string;
  claimSelectionId?: string;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { connections, connected, loading, error, refetch } =
    useGitConnections();
  // Same tab: the Settings page holds no unsaved input, and it is where the
  // callback lands anyway.
  const { connect, busy: connecting } = useConnectGitAccount();
  const { disconnect, busy: disconnecting } = useDisconnectGit();
  const forgetCachedRepos = useForgetCachedRepos();
  const selection = useClaimSelection(claimSelectionId);
  // The selection this page has finished with (chosen or cancelled). The
  // chooser closes at once; the URL drops the id through a replace navigation.
  const [closedSelection, setClosedSelection] = useState<string | null>(null);
  const showChooser =
    claimSelectionId != null && claimSelectionId !== closedSelection;

  function closeChooser() {
    setClosedSelection(claimSelectionId ?? null);
    void navigate({
      to: "/settings",
      search: (prev) => ({ ...prev, git_claim_selection: undefined }),
      hash: "integrations",
      replace: true,
    });
  }

  async function handleSelect(candidate: ClaimCandidate) {
    if (!(await selection.select(candidate.installationId))) return;
    forgetCachedRepos();
    closeChooser();
    toast.success(
      t("git.claimConnectedToast", { account: candidate.accountLogin }),
    );
    await refetch();
  }

  // Refetch when the tab regains focus — the GitHub callback redirects here.
  useEffect(() => {
    const onFocus = () => void refetch();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refetch]);

  const unavailable = isGitHubUnavailable(error);
  const initialLoading = loading && connections.length === 0 && !error;

  async function handleDisconnect(installationId: number) {
    const ok = await disconnect(installationId);
    if (!ok) return;
    forgetCachedRepos();
    await refetch();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <GithubIcon className="size-4" />
          {t("git.title")}
        </CardTitle>
        <CardDescription>{t("git.description")}</CardDescription>
        {connected && (
          <CardAction>
            <Badge variant="secondary">{t("git.connectedBadge")}</Badge>
          </CardAction>
        )}
      </CardHeader>
      <CardContent className="space-y-4">
        {showChooser && !unavailable && (
          <AccountChooser
            selection={selection}
            onSelect={handleSelect}
            onCancel={closeChooser}
          />
        )}
        {callbackError && !unavailable && (
          <Alert
            variant={
              callbackError === INSTALL_REQUESTED ? "default" : "destructive"
            }
          >
            <AlertTriangle />
            <AlertTitle>{t("git.callbackErrorTitle")}</AlertTitle>
            <AlertDescription>
              {callbackErrorMessage(t, callbackError)}
              {callbackError !== INSTALL_REQUESTED && (
                <RestartButton onClick={connect} disabled={connecting} />
              )}
            </AlertDescription>
          </Alert>
        )}
        {unavailable ? (
          <PanelCenteredState
            icon={<ShieldAlert />}
            title={t("git.unavailableTitle")}
            body={t("git.unavailableBody")}
          />
        ) : error && connections.length === 0 ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("git.errorTitle")}
            body={t("git.errorBody")}
          />
        ) : initialLoading ? (
          <GitConnectionsSkeleton />
        ) : connected ? (
          <ConnectedList
            connections={connections}
            onDisconnect={handleDisconnect}
            disconnecting={disconnecting}
            onConnectAnother={connect}
            connecting={connecting}
          />
        ) : (
          <DisconnectedState onConnect={connect} connecting={connecting} />
        )}
      </CardContent>
    </Card>
  );
}

/** Starts the single connect entry again from an alert. */
function RestartButton({
  onClick,
  disabled,
}: {
  onClick: () => void;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  return (
    <Button
      variant="outline"
      size="sm"
      className="mt-2"
      onClick={onClick}
      disabled={disabled}
    >
      <GithubIcon className="size-4" />
      {t("git.connectButton")}
    </Button>
  );
}

function DisconnectedState({
  onConnect,
  connecting,
}: {
  onConnect: () => void;
  connecting: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-col items-start gap-3">
      <p className="text-sm text-muted-foreground">
        {t("git.disconnectedBody")}
      </p>
      <Button onClick={onConnect} disabled={connecting}>
        <GithubIcon className="size-4" />
        {t("git.connectButton")}
      </Button>
    </div>
  );
}

function ConnectedList({
  connections,
  onDisconnect,
  disconnecting,
  onConnectAnother,
  connecting,
}: {
  connections: GitConnectionRow[];
  onDisconnect: (installationId: number) => void;
  disconnecting: boolean;
  onConnectAnother: () => void;
  connecting: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-4">
      <ul className="divide-y rounded-md border">
        {connections.map((c) => (
          <li key={c.installationId}>
            <ConnectionRow
              connection={c}
              onDisconnect={() => onDisconnect(c.installationId)}
              disconnecting={disconnecting}
            />
          </li>
        ))}
      </ul>
      <Button
        variant="outline"
        size="sm"
        onClick={onConnectAnother}
        disabled={connecting}
      >
        <GithubIcon className="size-4" />
        {t("git.connectAnotherButton")}
      </Button>
    </div>
  );
}

function ConnectionRow({
  connection,
  onDisconnect,
  disconnecting,
}: {
  connection: GitConnectionRow;
  onDisconnect: () => void;
  disconnecting: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 p-3">
      <div className="flex items-center gap-2 text-sm">
        <PlugZap className="size-4 text-muted-foreground" />
        <span>
          {t("git.connectedAs")}{" "}
          <span className="font-medium text-foreground">
            {connection.accountLogin}
          </span>
        </span>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {connection.installUrl && (
          <Button variant="outline" size="sm" asChild>
            <a href={connection.installUrl} target="_blank" rel="noreferrer">
              {t("git.manageAccess")}
            </a>
          </Button>
        )}
        <ConfirmDialog
          open={open}
          onOpenChange={setOpen}
          trigger={
            <Button variant="destructive" size="sm" disabled={disconnecting}>
              {t("git.disconnectButton")}
            </Button>
          }
          title={t("git.disconnectConfirmTitle")}
          description={t("git.disconnectConfirmBody")}
          cancelLabel={t("git.cancel")}
          confirmLabel={t("git.disconnectButton")}
          pending={disconnecting}
          onConfirm={() => {
            setOpen(false);
            onDisconnect();
          }}
        />
      </div>
    </div>
  );
}

/**
 * The account chooser every connect lands on (ADR078 §3b, w1/m526).
 *
 * The callback proved every installed account the user administers and
 * deferred the choice here, even for none or one: the user may have meant to
 * install on a new account instead. Accounts this workspace already holds are
 * shown marked and are not offered. Choosing spends the selection; a stale or
 * already used one degrades to "start again", never a silent dead end.
 */
function AccountChooser({
  selection: { candidates, loading, gone, selecting },
  onSelect,
  onCancel,
}: {
  selection: UseClaimSelectionResult;
  onSelect: (candidate: ClaimCandidate) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslations();
  const { connect: restart, busy: restarting } = useConnectGitAccount();
  // Install-first: the one remaining use of GitHub's install URL (ADR078 §3b).
  const { connect: install, busy: installing } = useConnectGit();
  if (loading) {
    // Structural preview of the ready state below: same bordered panel, the
    // title and body lines, one row per option at the row's own height, and
    // the action row.
    return (
      <div
        className="space-y-3 rounded-md border p-4"
        data-testid="git-account-chooser-skeleton"
      >
        <div>
          <span className="flex h-5 items-center">
            <Skeleton className="h-4 w-48" />
          </span>
          <span className="flex h-5 items-center">
            <Skeleton className="h-3.5 w-full" />
          </span>
        </div>
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
        <div className="flex flex-wrap items-center gap-2">
          <Skeleton className="h-8 w-60" />
          <Skeleton className="h-8 w-16" />
        </div>
      </div>
    );
  }
  if (gone) {
    return (
      <Alert>
        <AlertTriangle />
        <AlertTitle>{t("git.claimSelectionGoneTitle")}</AlertTitle>
        <AlertDescription>
          {t("git.claimSelectionGoneBody")}
          <RestartButton onClick={restart} disabled={restarting} />
        </AlertDescription>
      </Alert>
    );
  }
  return (
    <div
      className="space-y-3 rounded-md border p-4"
      data-testid="git-account-chooser"
    >
      <div>
        <p className="text-sm font-medium">{t("git.claimSelectionTitle")}</p>
        <p className="text-sm text-muted-foreground">
          {candidates.length > 0
            ? t("git.claimSelectionBody")
            : t("git.claimSelectionEmpty")}
        </p>
      </div>
      {candidates.length > 0 && (
        <ul className="space-y-2">
          {candidates.map((c) => (
            <li key={c.installationId}>
              {c.connected ? (
                <div className="flex h-9 items-center justify-between gap-2 rounded-md border px-4 text-sm text-muted-foreground">
                  <span className="flex min-w-0 items-center gap-2">
                    <GithubIcon className="size-4 shrink-0" />
                    <span className="truncate">{c.accountLogin}</span>
                  </span>
                  <Badge variant="secondary">{t("git.connectedBadge")}</Badge>
                </div>
              ) : (
                <Button
                  variant="outline"
                  className="w-full justify-start"
                  disabled={selecting}
                  onClick={() => onSelect(c)}
                >
                  <GithubIcon className="size-4" />
                  {c.accountLogin}
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={install}
          disabled={installing || selecting}
        >
          <Plus className="size-4" />
          {t("git.installAnotherButton")}
        </Button>
        <Button variant="ghost" size="sm" onClick={onCancel}>
          {t("git.cancel")}
        </Button>
      </div>
    </div>
  );
}
