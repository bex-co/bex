import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";
import type {
  LoginRequirementImpactKey,
  LoginRequirementImpactState,
} from "@/features/workspaces/hooks/use-login-requirement-impact";

/** The skeleton's rows: a representative count for the region's height. */
const SKELETON_ROWS = 3;

/**
 * Who turning a login requirement on would block (w7/m471,
 * docs/ADR012-auth.md 9c), inside the confirmation of a workspace's card and
 * of the org Security card: each member who loses access, or keeps it while
 * their listed keys stop working, then the API keys no member created. The
 * list is bex-api's, decided by the enforcement gate's own checks, after
 * Render's SSO enforcement list ("Each listed member will lose access", "Each
 * listed API key will be invalidated"); SSH keys are bex's addition, because
 * bex's gate blocks them too.
 *
 * Loading, failed, empty and listed all draw inside one region of fixed
 * height that scrolls, so the dialog keeps its geometry as the answer
 * arrives. A failed check says so and leaves confirming to the admin: it is
 * never shown as nobody. For Google login one line below says that members
 * signed in another way right now are asked to sign in with Google, which
 * the list cannot know.
 */
export function LoginRequirementImpactList({
  requirement,
  state,
  showWorkspace = false,
  membersNote = null,
}: {
  requirement: WorkspaceLoginRequirement;
  state: LoginRequirementImpactState;
  /** Name each API key's workspace (an org's list). */
  showWorkspace?: boolean;
  /** A line under the members' sentence: a workspace's 2FA confirmation
   *  says the reminders it will send (w1/m647). */
  membersNote?: string | null;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];

  return (
    // min-w-0: the dialog lays its children out in a grid, whose items keep
    // their content's width by default, so a fingerprint or a long email
    // would widen the dialog instead of truncating.
    <div className="min-w-0 space-y-2">
      <div
        role="region"
        aria-label={t("workspaces.loginImpactRegionLabel")}
        aria-busy={state.status === "loading"}
        data-login-requirement-impact={state.status}
        className="h-48 overflow-y-auto rounded-md border text-sm"
      >
        {state.status === "loading" ? (
          <ImpactSkeleton label={t("workspaces.loginImpactChecking")} />
        ) : state.status === "error" ? (
          <p role="status" className="text-muted-foreground p-3">
            {t("workspaces.loginImpactError")}
          </p>
        ) : state.members.length === 0 &&
          state.unattributedApiKeys.length === 0 ? (
          <p className="text-muted-foreground p-3">
            {t("workspaces.loginImpactNobody")}
          </p>
        ) : (
          <div className="divide-y">
            {state.members.length > 0 ? (
              <section className="space-y-1 py-2">
                <p className="text-muted-foreground px-3 text-xs">
                  {t(setting.impactMembersKey)}
                </p>
                {membersNote ? (
                  <p className="text-muted-foreground px-3 text-xs">
                    {membersNote}
                  </p>
                ) : null}
                <ul className="divide-y">
                  {state.members.map((member) => (
                    <li key={member.userId} className="space-y-0.5 px-3 py-2">
                      <div className="flex items-center justify-between gap-2">
                        <span className="min-w-0 truncate font-medium">
                          {member.name || member.email || member.userId}
                        </span>
                        <Badge
                          variant={
                            member.losesAccess ? "destructive" : "secondary"
                          }
                        >
                          {t(
                            member.losesAccess
                              ? "workspaces.loginImpactLosesAccess"
                              : "workspaces.loginImpactKeepsAccess",
                          )}
                        </Badge>
                      </div>
                      {member.name && member.email ? (
                        <p className="text-muted-foreground truncate text-xs">
                          {member.email}
                        </p>
                      ) : null}
                      {member.apiKeys.map((key) => (
                        <p key={key.id} className="truncate text-xs">
                          {apiKeyLine(t, key, showWorkspace)}
                        </p>
                      ))}
                      {member.sshKeys.map((key) => (
                        <p key={key.id} className="truncate text-xs">
                          {t("workspaces.loginImpactSshKey", {
                            name: key.name,
                          })}{" "}
                          <span className="text-muted-foreground font-mono">
                            {key.fingerprint}
                          </span>
                        </p>
                      ))}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
            {state.unattributedApiKeys.length > 0 ? (
              <section className="space-y-1 px-3 py-2">
                <p className="text-muted-foreground text-xs">
                  {t("workspaces.loginImpactUnattributed")}
                </p>
                <ul>
                  {state.unattributedApiKeys.map((key) => (
                    <li key={key.id} className="truncate text-xs">
                      {apiKeyLine(t, key, showWorkspace)}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
          </div>
        )}
      </div>
      {requirement === "googleLogin" ? (
        <p className="text-muted-foreground text-xs">
          {t("workspaces.loginImpactGoogleSessions")}
        </p>
      ) : null}
    </div>
  );
}

function apiKeyLine(
  t: Translate,
  key: LoginRequirementImpactKey,
  showWorkspace: boolean,
): string {
  return showWorkspace && key.workspaceName
    ? t("workspaces.loginImpactApiKeyInWorkspace", {
        name: key.name,
        workspace: key.workspaceName,
      })
    : t("workspaces.loginImpactApiKey", { name: key.name });
}

/** The listed state's first rows, while the check is in flight. */
function ImpactSkeleton({ label }: { label: string }) {
  return (
    <div className="space-y-1 py-2">
      <span className="sr-only">{label}</span>
      <Skeleton className="mx-3 h-3 w-3/4" />
      <ul className="divide-y">
        {Array.from({ length: SKELETON_ROWS }, (_, i) => (
          <li key={i} className="space-y-1.5 px-3 py-2">
            <div className="flex items-center justify-between gap-2">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-5 w-20" />
            </div>
            <Skeleton className="h-3 w-48" />
          </li>
        ))}
      </ul>
    </div>
  );
}
