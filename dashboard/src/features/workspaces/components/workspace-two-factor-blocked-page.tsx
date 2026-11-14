import { Link, useNavigate, useRouter } from "@tanstack/react-router";
import { Loader2, LogOut, ShieldAlert } from "lucide-react";
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
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useLeaveWorkspace } from "@/features/team/hooks/use-leave-workspace";
import { WorkspaceTwoFactorBlockedSkeleton } from "@/features/workspaces/components/workspace-two-factor-blocked-skeleton";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { clearWorkspaceTwoFactorBlocks } from "@/features/workspaces/lib/two-factor-block";

/**
 * The one page a member sees when bex-api refuses their workspace with
 * `WORKSPACE_2FA_REQUIRED` (w22/m402, docs/ADR012-auth.md 9a): which workspace,
 * why, and the three ways out. Enrollment is Kratos's own settings flow, so
 * the page links to account security rather than re-implementing it; the
 * other two exits (switch workspace, leave this one) only touch the workspace
 * list and `leaveWorkspace`, both of which the server exempts from the block,
 * so the page cannot trap its reader.
 *
 * Access returns within the server's 60 s cache of the enrollment fact after
 * the member enables 2FA, so "check again" drops the recorded block and lets
 * the page's own queries decide; if the server still refuses, this page comes
 * straight back.
 */
export function WorkspaceTwoFactorBlockedPage({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { t } = useTranslations();
  const router = useRouter();
  const navigate = useNavigate();
  const { workspaces, currentWorkspace, setCurrentWorkspaceId } =
    useWorkspace();
  const { leave, busy, error } = useLeaveWorkspace(workspaceId);
  const others = workspaces.filter((w) => w.id !== workspaceId);

  // The name arrives with the workspace list; a block can be recorded first.
  if (!currentWorkspace) return <WorkspaceTwoFactorBlockedSkeleton />;
  const name = currentWorkspace.name;

  function recheck() {
    clearWorkspaceTwoFactorBlocks();
    void router.invalidate();
  }

  function switchTo(id: string) {
    setCurrentWorkspaceId(id);
    // The route in view belongs to the workspace being left.
    void navigate({ to: "/", replace: true });
  }

  return (
    <div
      className="flex-1 overflow-auto p-4 sm:p-6"
      data-testid="workspace-two-factor-blocked"
    >
      <div className="mx-auto w-full max-w-xl">
        <Card>
          <CardHeader>
            <CardTitle as="h1" className="flex items-center gap-2">
              <ShieldAlert className="size-5 shrink-0 text-destructive" />
              {t("workspaces.twoFactorBlockedTitle", { workspace: name })}
            </CardTitle>
            <CardDescription>
              {t("workspaces.twoFactorBlockedBody")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <section data-region="enroll" className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <Button asChild>
                  <Link to="/settings" hash="security">
                    {t("workspaces.twoFactorEnrollLink")}
                  </Link>
                </Button>
                <Button variant="outline" onClick={recheck}>
                  {t("workspaces.twoFactorBlockedRecheck")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {t("workspaces.twoFactorBlockedRecheckHint")}
              </p>
            </section>
            <section data-region="switch" className="space-y-2">
              <Label id="two-factor-blocked-switch">
                {t("workspaces.twoFactorBlockedSwitchLabel")}
              </Label>
              <Select
                value=""
                disabled={others.length === 0}
                onValueChange={switchTo}
              >
                <SelectTrigger
                  className="w-full sm:w-72"
                  aria-labelledby="two-factor-blocked-switch"
                >
                  <SelectValue
                    placeholder={t(
                      others.length === 0
                        ? "workspaces.twoFactorBlockedNoOtherWorkspaces"
                        : "workspaces.twoFactorBlockedSwitchPlaceholder",
                    )}
                  />
                </SelectTrigger>
                <SelectContent>
                  {others.map((w) => (
                    <SelectItem key={w.id} value={w.id}>
                      {w.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </section>
            <section data-region="leave" className="space-y-3">
              {error ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("team.leaveErrorTitle")}</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              ) : null}
              <p className="text-sm text-muted-foreground">
                {t("team.leaveDescription", { workspace: name })}
              </p>
              <ConfirmDialog
                trigger={
                  <Button
                    variant="outline"
                    className="text-destructive"
                    disabled={busy}
                  >
                    {busy ? <Loader2 className="animate-spin" /> : <LogOut />}
                    {t("team.leaveAction")}
                  </Button>
                }
                title={t("team.leaveConfirmTitle")}
                description={t("team.leaveConfirm", { workspace: name })}
                cancelLabel={t("team.leaveCancel")}
                confirmLabel={t("team.leaveAction")}
                onConfirm={() => void leave()}
              />
            </section>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
