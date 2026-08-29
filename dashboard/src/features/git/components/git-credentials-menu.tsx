import { useEffect, useMemo, useState } from "react";
import {
  Github,
  ChevronDown,
  ExternalLink,
  Settings2,
  Unplug,
  Plus,
  ShieldAlert,
  AlertTriangle,
} from "lucide-react";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/common/components/ui/popover";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useGitConnections,
  type GitConnectionRow,
} from "@/features/git/hooks/use-git-connection";
import { useConnectGit } from "@/features/git/hooks/use-connect-git";
import { useClaimGit } from "@/features/git/hooks/use-claim-git";
import { useDisconnectGit } from "@/features/git/hooks/use-disconnect-git";
import { useRepos } from "@/features/services/hooks/use-repos";
import { isGitHubUnavailable } from "@/features/git/lib/errors";

/**
 * In-place GitHub credentials control for the service source picker (w8/m31),
 * mirroring Render's `/web/new` "Credentials (N)" dropdown: a trigger showing the
 * connected-account count and a popover listing every connected GitHub
 * account/org — each with its repo count, an "Open in GitHub" link, a "Configure
 * in GitHub" (grants) link, and a Disconnect — plus "Connect another account"
 * and the ADR075 §3a claim fallback.
 *
 * Unlike the Settings card, connecting opens the install in a NEW TAB
 * (`useConnectGit({ newTab: true })`) so a half-filled create form survives; the
 * menu refetches connections AND repos on window focus, so a newly-granted
 * account's repos appear in place when the user returns.
 */
export function GitCredentialsMenu() {
  const { t } = useTranslations();
  const { connections, connected, loading, error, refetch } =
    useGitConnections();
  const { repos, refetch: refetchRepos } = useRepos();
  const { connect, busy: connecting } = useConnectGit({ newTab: true });
  const { claim, busy: claiming } = useClaimGit();
  const { disconnect, busy: disconnecting } = useDisconnectGit();
  const [pendingDisconnect, setPendingDisconnect] =
    useState<GitConnectionRow | null>(null);

  // The connect flow opens a new tab; focus is the signal the user came back.
  // Refresh both the connection set and the repo list so the picker updates.
  useEffect(() => {
    const onFocus = () => {
      void refetch();
      void refetchRepos();
    };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refetch, refetchRepos]);

  const unavailable = isGitHubUnavailable(error);

  // Repos carry accountLogin (ADR075); an account has one installation per app,
  // so login uniquely identifies a connection within a workspace.
  const repoCountByAccount = useMemo(() => {
    const counts = new Map<string, number>();
    for (const r of repos) {
      counts.set(r.accountLogin, (counts.get(r.accountLogin) ?? 0) + 1);
    }
    return counts;
  }, [repos]);

  async function handleDisconnect(installationId: number) {
    const ok = await disconnect(installationId);
    if (ok) {
      await refetch();
      await refetchRepos();
    }
  }

  const initialLoading = loading && connections.length === 0 && !error;

  return (
    <>
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="outline" size="sm" className="gap-1.5">
            <Github className="size-4" />
            {t("git.credentialsTrigger", { count: connections.length })}
            <ChevronDown className="size-3.5 text-muted-foreground" />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-80 p-0">
          {unavailable ? (
            <MenuMessage
              icon={<ShieldAlert className="size-4" />}
              title={t("git.unavailableTitle")}
              body={t("git.unavailableBody")}
            />
          ) : error && connections.length === 0 ? (
            <MenuMessage
              icon={<AlertTriangle className="size-4" />}
              title={t("git.errorTitle")}
              body={t("git.errorBody")}
            />
          ) : initialLoading ? (
            <div className="space-y-2 p-3">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : (
            <div className="flex flex-col">
              {connected ? (
                <>
                  <p className="px-3 pt-3 pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                    {t("git.credentialsAccountsHeading")}
                  </p>
                  <ul className="max-h-64 divide-y overflow-y-auto">
                    {connections.map((c) => (
                      <li key={c.installationId}>
                        <AccountRow
                          connection={c}
                          repoCount={repoCountByAccount.get(c.accountLogin)}
                          onDisconnect={() => setPendingDisconnect(c)}
                          disconnecting={disconnecting}
                        />
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                <p className="px-3 pt-3 pb-1 text-sm text-muted-foreground">
                  {t("git.disconnectedBody")}
                </p>
              )}
              <div className="flex flex-col gap-1 border-t p-2">
                <Button
                  variant="ghost"
                  size="sm"
                  className="justify-start"
                  onClick={connect}
                  disabled={connecting}
                >
                  <Plus className="size-4" />
                  {connected
                    ? t("git.connectAnotherButton")
                    : t("git.connectButton")}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="justify-start"
                  onClick={claim}
                  disabled={claiming}
                >
                  <Github className="size-4" />
                  {t("git.claimButton")}
                </Button>
              </div>
            </div>
          )}
        </PopoverContent>
      </Popover>

      <ConfirmDialog
        open={pendingDisconnect != null}
        onOpenChange={(next) => {
          if (!next) setPendingDisconnect(null);
        }}
        title={t("git.disconnectConfirmTitle")}
        description={t("git.disconnectConfirmBody")}
        cancelLabel={t("git.cancel")}
        confirmLabel={t("git.disconnectButton")}
        pending={disconnecting}
        onConfirm={() => {
          const target = pendingDisconnect;
          setPendingDisconnect(null);
          if (target) void handleDisconnect(target.installationId);
        }}
      />
    </>
  );
}

function AccountRow({
  connection,
  repoCount,
  onDisconnect,
  disconnecting,
}: {
  connection: GitConnectionRow;
  repoCount: number | undefined;
  onDisconnect: () => void;
  disconnecting: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex items-center justify-between gap-2 px-3 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <Github className="size-4 shrink-0 text-muted-foreground" />
        <a
          href={`https://github.com/${connection.accountLogin}`}
          target="_blank"
          rel="noreferrer"
          className="flex min-w-0 items-center gap-1 text-sm font-medium hover:underline"
          title={t("git.openInGitHub")}
        >
          <span className="truncate">{connection.accountLogin}</span>
          <ExternalLink className="size-3 shrink-0 text-muted-foreground" />
        </a>
        {repoCount != null && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {t("git.repoCount", { count: repoCount })}
          </span>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-0.5">
        {connection.installUrl && (
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            asChild
            title={t("git.configureInGitHub")}
          >
            <a
              href={connection.installUrl}
              target="_blank"
              rel="noreferrer"
              aria-label={t("git.configureInGitHub")}
            >
              <Settings2 className="size-4" />
            </a>
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon"
          className="size-7 text-destructive hover:text-destructive"
          onClick={onDisconnect}
          disabled={disconnecting}
          title={t("git.disconnectButton")}
          aria-label={t("git.disconnectAccount", {
            account: connection.accountLogin,
          })}
        >
          <Unplug className="size-4" />
        </Button>
      </div>
    </div>
  );
}

function MenuMessage({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <div className="flex flex-col items-start gap-1 p-3">
      <div className="flex items-center gap-2 text-sm font-medium">
        {icon}
        {title}
      </div>
      <p className="text-xs text-muted-foreground">{body}</p>
    </div>
  );
}
