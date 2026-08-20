import { useEffect, useState } from "react";
import { Github, PlugZap, ShieldAlert, AlertTriangle } from "lucide-react";
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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
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
import { useConnectGit } from "@/features/git/hooks/use-connect-git";
import { useClaimGit } from "@/features/git/hooks/use-claim-git";
import { useDisconnectGit } from "@/features/git/hooks/use-disconnect-git";

// The backend answers ErrGitHubUnavailable (503) when BEX_GITHUB_APP_* is unset;
// that message flows through GraphQL as "github integration not configured".
function isUnavailable(error: Error | undefined): boolean {
  if (!error) return false;
  const m = error.message.toLowerCase();
  return m.includes("not configured") || m.includes("unavailable");
}

// The bounded git_error codes the callback redirects with (backend
// internal/github/rest.go). missing_state is the direct-github.com-install case;
// GitHub strips the state for already-installed accounts (ADR075 §3a), so its
// recovery — and the claim flow's own bounded failures — point at the CLAIM
// action, never at retrying the install URL.
function callbackErrorMessage(t: (k: string) => string, code: string): string {
  switch (code) {
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

// The codes whose recovery is the claim flow (an installation already exists on
// GitHub; the install URL cannot bind it).
function claimRecovers(code: string | undefined): boolean {
  return code === "missing_state" || code === "ambiguous_installation";
}

/**
 * Settings → Connect GitHub (w2/m8, multi-account ADR075): lists every GitHub
 * account/org the workspace has connected — each with a Manage-access link and a
 * per-account Disconnect — plus a Connect (another) action that starts the
 * stateful install flow. Also renders the unavailable (no GitHub App) and error
 * states, and a callback-failure alert. Returning from GitHub's install callback
 * lands on /settings, so the card refetches on mount and on window focus.
 */
export function ConnectGithubCard({
  callbackError,
}: {
  callbackError?: string;
}) {
  const { t } = useTranslations();
  const { connections, connected, loading, error, refetch } =
    useGitConnections();
  const { connect, busy: connecting } = useConnectGit();
  const { claim, busy: claiming } = useClaimGit();
  const { disconnect, busy: disconnecting } = useDisconnectGit();

  // Refetch when the tab regains focus — the GitHub callback redirects here.
  useEffect(() => {
    const onFocus = () => void refetch();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refetch]);

  const unavailable = isUnavailable(error);
  const initialLoading = loading && connections.length === 0 && !error;

  async function handleDisconnect(installationId: number) {
    const ok = await disconnect(installationId);
    if (ok) await refetch();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Github className="size-4" />
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
        {callbackError && !unavailable && (
          <Alert variant="destructive">
            <AlertTriangle />
            <AlertTitle>{t("git.callbackErrorTitle")}</AlertTitle>
            <AlertDescription>
              {callbackErrorMessage(t, callbackError)}
              {claimRecovers(callbackError) && (
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-2"
                  onClick={claim}
                  disabled={claiming}
                >
                  <Github className="size-4" />
                  {t("git.claimButton")}
                </Button>
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
          <Skeleton className="h-10 w-full" />
        ) : connected ? (
          <ConnectedList
            connections={connections}
            onDisconnect={handleDisconnect}
            disconnecting={disconnecting}
            onConnectAnother={connect}
            connecting={connecting}
            onClaim={claim}
            claiming={claiming}
          />
        ) : (
          <DisconnectedState
            onConnect={connect}
            connecting={connecting}
            onClaim={claim}
            claiming={claiming}
          />
        )}
      </CardContent>
    </Card>
  );
}

function DisconnectedState({
  onConnect,
  connecting,
  onClaim,
  claiming,
}: {
  onConnect: () => void;
  connecting: boolean;
  onClaim: () => void;
  claiming: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-col items-start gap-3">
      <p className="text-sm text-muted-foreground">
        {t("git.disconnectedBody")}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={onConnect} disabled={connecting}>
          <Github className="size-4" />
          {t("git.connectButton")}
        </Button>
        <Button variant="outline" onClick={onClaim} disabled={claiming}>
          {t("git.claimButton")}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">{t("git.claimHint")}</p>
    </div>
  );
}

function ConnectedList({
  connections,
  onDisconnect,
  disconnecting,
  onConnectAnother,
  connecting,
  onClaim,
  claiming,
}: {
  connections: GitConnectionRow[];
  onDisconnect: (installationId: number) => void;
  disconnecting: boolean;
  onConnectAnother: () => void;
  connecting: boolean;
  onClaim: () => void;
  claiming: boolean;
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
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={onConnectAnother}
          disabled={connecting}
        >
          <Github className="size-4" />
          {t("git.connectAnotherButton")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={onClaim}
          disabled={claiming}
        >
          {t("git.claimButton")}
        </Button>
      </div>
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
      <div className="flex items-center gap-2">
        {connection.installUrl && (
          <Button variant="outline" size="sm" asChild>
            <a href={connection.installUrl} target="_blank" rel="noreferrer">
              {t("git.manageAccess")}
            </a>
          </Button>
        )}
        <AlertDialog open={open} onOpenChange={setOpen}>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" size="sm" disabled={disconnecting}>
              {t("git.disconnectButton")}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("git.disconnectConfirmTitle")}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("git.disconnectConfirmBody")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("git.cancel")}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => {
                  setOpen(false);
                  onDisconnect();
                }}
              >
                {t("git.disconnectButton")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </div>
  );
}
