import { useEffect, useMemo, useRef, useState } from "react";
import {
  ChevronDown,
  ExternalLink,
  Settings2,
  Unplug,
  Plus,
  ShieldAlert,
  AlertTriangle,
} from "lucide-react";
import { GithubIcon } from "@/common/components/github-icon";
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
import { useConnectGitAccount } from "@/features/git/hooks/use-connect-git";
import { useDisconnectGit } from "@/features/git/hooks/use-disconnect-git";
import { useRepos } from "@/features/services/hooks/use-repos";
import { isGitHubUnavailable } from "@/features/git/lib/errors";

/**
 * In-place GitHub credentials control for the service source picker (w8/m31),
 * mirroring Render's `/web/new` "Credentials (N)" dropdown: a trigger showing the
 * connected-account count and a popover listing every connected GitHub
 * account/org — each with its repo count, an "Open in GitHub" link, a "Configure
 * in GitHub" (grants) link, and a Disconnect — plus ONE connect action (ADR078
 * §3b, w1/m526) that starts with GitHub OAuth and lands on the account chooser,
 * so an account that already has the App connects without reinstalling.
 *
 * Unlike the Settings card, connecting runs in a NEW TAB
 * (`useConnectGitAccount({ newTab: true })`) so a half-filled create form
 * survives; the menu refetches connections AND repos on window focus, and the
 * source picker refetches repos whenever the connection set changes, so a
 * newly-connected account's repos appear in place when the user returns.
 */
export function GitCredentialsMenu({
  open,
  onOpenChange,
}: {
  /** Controlled by a caller that opens the menu from a control of its own. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  const { connections, connected, loading, error, refetch } =
    useGitConnections();
  const { repos, refetch: refetchRepos } = useRepos();
  const { connect, busy: connecting } = useConnectGitAccount({ newTab: true });
  const { disconnect, busy: disconnecting } = useDisconnectGit();
  const [pendingDisconnect, setPendingDisconnect] =
    useState<GitConnectionRow | null>(null);
  const connectRef = useRef<HTMLButtonElement>(null);

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
      <Popover open={open} onOpenChange={onOpenChange}>
        <PopoverTrigger asChild>
          {/* The search field's height and border beside it: the two are one
              row of controls. It says "Credentials (3)" at every width: a
              bare "3" beside a mark did not say what it counted, and the
              list's empty states send people to "Credentials" by name. */}
          <Button
            variant="outline"
            className="h-11 gap-1.5 border-muted-foreground/70 max-sm:grow sm:h-9 dark:border-muted-foreground/70"
          >
            {/* The word names it; on a phone the mark's width goes to the
                search field beside it. */}
            <GithubIcon className="size-4 max-sm:hidden" />
            {/* Until the accounts are read there is no count to show: a bar
                where it will be, not "Credentials (0)". The bar and its gap
                take the width " (3)" will, so the search beside it does not
                move when the count arrives. */}
            {initialLoading ? (
              <>
                <span>{t("git.credentialsMenuLabel")}</span>
                <Skeleton className="h-3.5 w-[17px]" />
              </>
            ) : (
              <span className="tabular-nums">
                {t("git.credentialsTrigger", { count: connections.length })}
              </span>
            )}
            <ChevronDown className="size-3.5 text-muted-foreground" />
          </Button>
        </PopoverTrigger>
        <PopoverContent
          align="end"
          // On a phone it spans the search row it hangs from (the dialog's or
          // page's column: the viewport less its margins and padding).
          className="w-80 p-0 max-sm:w-[calc(100vw-5rem)]"
          aria-label={t("git.credentialsMenuLabel")}
          // Radix focuses the first tabbable element but skips links, and each
          // row's account name and Configure entry are links, so it used to land
          // on a Disconnect button. Focus the connect action instead, or the
          // panel itself while it has none (loading, unavailable).
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            const panel = event.currentTarget as HTMLElement | null;
            (connectRef.current ?? panel)?.focus();
          }}
        >
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
                  <ul className="max-h-72 divide-y overflow-y-auto">
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
              <div className="flex flex-col border-t p-2">
                <Button
                  ref={connectRef}
                  variant="ghost"
                  size="sm"
                  className="h-11 justify-start sm:h-8"
                  onClick={connect}
                  disabled={connecting}
                >
                  <Plus className="size-4" />
                  {connected
                    ? t("git.connectAnotherButton")
                    : t("git.connectButton")}
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

// An account's two actions: 44 px tall on a phone, and padded so each label
// starts under the account's mark.
const ACCOUNT_ACTION_CLASS =
  "h-11 gap-1 px-1.5 has-[>svg]:px-1.5 sm:h-8 sm:gap-1.5 sm:px-2 sm:has-[>svg]:px-2";

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
    <div className="flex flex-col px-3 pb-1 sm:pt-2">
      {/* On a phone the name's link is a 44 px target: it reaches up into the
          space that sets this account apart from the one above. */}
      <div className="flex items-center gap-2 max-sm:mt-5 max-sm:mb-1">
        <GithubIcon className="size-4 shrink-0 text-muted-foreground" />
        <a
          href={`https://github.com/${connection.accountLogin}`}
          target="_blank"
          rel="noreferrer"
          className="relative flex min-w-0 items-center gap-1 text-sm font-medium hover:underline max-sm:after:absolute max-sm:after:inset-x-0 max-sm:after:-top-5 max-sm:after:-bottom-1 max-sm:after:content-['']"
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
      {/* Each action is named, on one line under the account: two bare glyphs
          said nothing, least of all that one of them disconnects. Disconnect
          is quiet until pointed at: three red labels in a row read as three
          alarms. The negative margin sets each label's left edge under the
          account's mark. A phone drops the glyphs so both names fit one line
          at 360 px; a language too long for that wraps them onto a line each
          instead of running off the menu. */}
      <div className="-mx-1.5 flex flex-wrap items-center justify-between sm:-mx-2">
        {connection.installUrl && (
          <Button
            variant="ghost"
            size="sm"
            className={ACCOUNT_ACTION_CLASS}
            asChild
          >
            <a href={connection.installUrl} target="_blank" rel="noreferrer">
              <Settings2 className="size-4 max-sm:hidden" />
              {t("git.configureInGitHub")}
            </a>
          </Button>
        )}
        <Button
          variant="ghost"
          size="sm"
          className={`${ACCOUNT_ACTION_CLASS} text-muted-foreground hover:text-destructive focus-visible:text-destructive`}
          onClick={onDisconnect}
          disabled={disconnecting}
          aria-label={t("git.disconnectAccount", {
            account: connection.accountLogin,
          })}
        >
          <Unplug className="size-4 max-sm:hidden" />
          {t("git.disconnectButton")}
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
