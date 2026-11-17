import type { ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Loader2, LogOut, ShieldAlert } from "lucide-react";
import { PageScroll } from "@/common/components/page-frame";
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
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * The frame both blocked pages share (2FA, w22/m402; Google login, w7/m465):
 * one card naming the workspace and why, its first region the requirement's
 * own way back in, then `WorkspaceBlockedExits`.
 * `WorkspaceBlockedFrameSkeleton` is its pending shape.
 */
export function WorkspaceBlockedFrame({
  testId,
  title,
  description,
  children,
}: {
  testId: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <PageScroll data-testid={testId}>
      <div className="mx-auto w-full max-w-xl">
        <Card>
          <CardHeader>
            <CardTitle as="h1" className="flex items-center gap-2">
              <ShieldAlert className="size-5 shrink-0 text-destructive" />
              {title}
            </CardTitle>
            <CardDescription>{description}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">{children}</CardContent>
        </Card>
      </div>
    </PageScroll>
  );
}

/**
 * The two exits every blocked page offers besides meeting the requirement:
 * switch to another workspace, or leave this one. Both only touch the
 * workspace list and `leaveWorkspace`, which the server exempts from every
 * login requirement, so the page cannot trap its reader.
 */
export function WorkspaceBlockedExits({
  workspaceId,
  name,
}: {
  workspaceId: string;
  name: string;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { workspaces, setCurrentWorkspaceId } = useWorkspace();
  const { leave, busy, error } = useLeaveWorkspace(workspaceId);
  const others = workspaces.filter((w) => w.id !== workspaceId);

  function switchTo(id: string) {
    setCurrentWorkspaceId(id);
    // The route in view belongs to the workspace being left.
    void navigate({ to: "/", replace: true });
  }

  return (
    <>
      <section data-region="switch" className="space-y-2">
        <Label id="workspace-blocked-switch">
          {t("workspaces.loginBlockedSwitchLabel")}
        </Label>
        <Select
          value=""
          disabled={others.length === 0}
          onValueChange={switchTo}
        >
          <SelectTrigger
            className="w-full sm:w-72"
            aria-labelledby="workspace-blocked-switch"
          >
            <SelectValue
              placeholder={t(
                others.length === 0
                  ? "workspaces.loginBlockedNoOtherWorkspaces"
                  : "workspaces.loginBlockedSwitchPlaceholder",
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
    </>
  );
}
