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
import { useTranslations } from "@/common/hooks/use-translations";
import { useGitConnection } from "@/features/git/hooks/use-git-connection";
import { useConnectGit } from "@/features/git/hooks/use-connect-git";
import { useDisconnectGit } from "@/features/git/hooks/use-disconnect-git";

// The backend answers ErrGitHubUnavailable (503) when BEX_GITHUB_APP_* is unset;
// that message flows through GraphQL as "github integration not configured".
function isUnavailable(error: Error | undefined): boolean {
  if (!error) return false;
  const m = error.message.toLowerCase();
  return m.includes("not configured") || m.includes("unavailable");
}

/**
 * Settings → Connect GitHub (w2/m8): shows the workspace's GitHub App connection
 * — disconnected (Connect → GitHub install screen), connected (account login +
 * Disconnect + a link to manage repo grants on GitHub), or unavailable (backend
 * has no GitHub App configured). Returning from GitHub's install callback lands
 * on /settings, so the card refetches on mount and on window focus to show the
 * fresh connection without a manual reload.
 */
export function ConnectGithubCard() {
  const { t } = useTranslations();
  const { connection, loading, error, refetch } = useGitConnection();
  const { connect, busy: connecting } = useConnectGit();
  const { disconnect, busy: disconnecting } = useDisconnectGit();

  // Refetch when the tab regains focus — the GitHub callback redirects here.
  useEffect(() => {
    const onFocus = () => void refetch();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refetch]);

  const unavailable = isUnavailable(error);
  const initialLoading = loading && !connection && !error;

  async function handleDisconnect() {
    const ok = await disconnect();
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
        {connection?.connected && (
          <CardAction>
            <Badge variant="secondary">{t("git.connectedBadge")}</Badge>
          </CardAction>
        )}
      </CardHeader>
      <CardContent>
        {unavailable ? (
          <PanelCenteredState
            icon={<ShieldAlert />}
            title={t("git.unavailableTitle")}
            body={t("git.unavailableBody")}
          />
        ) : error && !connection ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("git.errorTitle")}
            body={t("git.errorBody")}
          />
        ) : initialLoading ? (
          <Skeleton className="h-10 w-full" />
        ) : connection?.connected ? (
          <ConnectedState
            accountLogin={connection.accountLogin}
            installUrl={connection.installUrl}
            onDisconnect={handleDisconnect}
            disconnecting={disconnecting}
          />
        ) : (
          <DisconnectedState onConnect={connect} connecting={connecting} />
        )}
      </CardContent>
    </Card>
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
        <Github className="size-4" />
        {t("git.connectButton")}
      </Button>
    </div>
  );
}

function ConnectedState({
  accountLogin,
  installUrl,
  onDisconnect,
  disconnecting,
}: {
  accountLogin: string;
  installUrl: string;
  onDisconnect: () => void;
  disconnecting: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-2 text-sm">
        <PlugZap className="size-4 text-muted-foreground" />
        <span>
          {t("git.connectedAs")}{" "}
          <span className="font-medium text-foreground">{accountLogin}</span>
        </span>
      </div>
      <div className="flex items-center gap-2">
        {installUrl && (
          <Button variant="outline" size="sm" asChild>
            <a href={installUrl} target="_blank" rel="noreferrer">
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
