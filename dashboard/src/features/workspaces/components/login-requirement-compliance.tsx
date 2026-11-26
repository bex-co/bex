import { useState } from "react";
import { CircleCheck, Loader2, TriangleAlert } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { cn } from "@/common/lib/utils/utils";
import {
  useSendTwoFactorReminders,
  useWorkspaceNonCompliantMembers,
  type NonCompliantMemberView,
} from "@/features/workspaces/hooks/use-workspace-non-compliant-members";
import { LOGIN_REQUIREMENT_SETTINGS } from "@/features/workspaces/lib/login-requirements";

/**
 * Under a login requirement the workspace enforces, what Render's Workspace
 * Settings shows its admins (w1/m647,
 * docs/render-artifacts/workspace-2fa-reminders.md): how many members do not
 * meet it, with "View members", or that all of them do. The members dialog
 * lists them, and for 2FA sends Render's email reminders, at most once per
 * workspace per UTC day; Google login has the list only, as on Render.
 *
 * The list is bex-api's, by the enforcement gate's own check. A read that
 * fails shows nothing, as Render's section does, never "All members".
 * `pending` holds the notice's box while the caller's role is still being
 * checked: an admin's card will draw it.
 */
export function LoginRequirementCompliance({
  requirement,
  workspaceId,
  pending = false,
}: {
  requirement: WorkspaceLoginRequirement;
  workspaceId: string;
  pending?: boolean;
}) {
  const state = useWorkspaceNonCompliantMembers(
    workspaceId,
    requirement,
    !pending,
  );
  if (pending || state.status === "loading") {
    return <LoginRequirementNotice requirement={requirement} pending />;
  }
  if (state.status === "error") return null;
  return (
    <NonCompliantNotice
      requirement={requirement}
      workspaceId={workspaceId}
      members={state.members}
    />
  );
}

function NonCompliantNotice({
  requirement,
  workspaceId,
  members,
}: {
  requirement: WorkspaceLoginRequirement;
  workspaceId: string;
  members: readonly NonCompliantMemberView[];
}) {
  const [listOpen, setListOpen] = useState(false);
  const [limitOpen, setLimitOpen] = useState(false);
  return (
    <>
      <LoginRequirementNotice
        requirement={requirement}
        count={members.length}
        onViewMembers={() => setListOpen(true)}
      />
      {members.length > 0 ? (
        <NonCompliantMembersDialog
          requirement={requirement}
          workspaceId={workspaceId}
          members={members}
          open={listOpen}
          onOpenChange={setListOpen}
          onLimit={() => setLimitOpen(true)}
        />
      ) : null}
      <ReminderLimitDialog open={limitOpen} onOpenChange={setLimitOpen} />
    </>
  );
}

/** The notice's box, the same in the ready card and its placeholder. */
const NOTICE_CLASS =
  "flex items-center gap-3 rounded-lg border px-4 py-3 text-sm";

/**
 * Render's notice: a warning naming how many members do not meet the
 * requirement, in bold, with "View members"; or, with `count` 0, the success
 * line naming "All members". `pending` draws the same box, words and button
 * without ink, for a count the page does not know yet (0 unless told):
 * once the read answers, the box holds the shape it will have.
 */
export function LoginRequirementNotice({
  requirement,
  count = 0,
  pending = false,
  onViewMembers,
}: {
  requirement: WorkspaceLoginRequirement;
  count?: number;
  pending?: boolean;
  onViewMembers?: () => void;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  const missing = count > 0;
  const [before, after] = aroundPlaceholder(
    missing
      ? t(setting.noticeKey, { count, members: PLACEHOLDER_SENTINEL })
      : t(setting.allCompliantKey, { members: PLACEHOLDER_SENTINEL }),
  );
  const sentence = (
    <>
      {before}
      <strong className="font-semibold">
        {missing
          ? t("workspaces.loginNoticeMembers", { count })
          : t("workspaces.loginNoticeAllMembers")}
      </strong>
      {after}
    </>
  );
  const view = t("workspaces.loginNoticeViewMembers", { count });
  const Icon = missing ? TriangleAlert : CircleCheck;
  return (
    <div
      role={pending ? undefined : "status"}
      aria-hidden={pending || undefined}
      data-login-requirement-notice={
        pending ? "pending" : missing ? "missing" : "all"
      }
      data-skeleton-region={pending ? `${setting.section}-notice` : undefined}
      className={cn(
        NOTICE_CLASS,
        missing
          ? "border-amber-500/40 bg-amber-500/10"
          : "border-emerald-500/40 bg-emerald-500/10",
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn(
          "size-4 shrink-0",
          pending && "invisible",
          missing
            ? "text-amber-600 dark:text-amber-400"
            : "text-emerald-600 dark:text-emerald-400",
        )}
      />
      <div className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p className="relative min-w-0 flex-[1_1_16rem]">
          {pending ? <ReservedText>{sentence}</ReservedText> : sentence}
        </p>
        {missing ? (
          pending ? (
            <ReservedButton variant="outline" size="sm">
              {view}
            </ReservedButton>
          ) : (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onViewMembers}
            >
              {view}
            </Button>
          )
        ) : null}
      </div>
    </div>
  );
}

/**
 * Render's member dialog: the members who do not meet the requirement, and
 * for 2FA its reminder callout. A batch sent closes it with Render's toast; a
 * batch already sent today closes it for the limit dialog; any other failure
 * stays here with Render's apology.
 */
function NonCompliantMembersDialog({
  requirement,
  workspaceId,
  members,
  open,
  onOpenChange,
  onLimit,
}: {
  requirement: WorkspaceLoginRequirement;
  workspaceId: string;
  members: readonly NonCompliantMemberView[];
  open: boolean;
  onOpenChange: (next: boolean) => void;
  onLimit: () => void;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  const { send, sending } = useSendTwoFactorReminders(workspaceId);
  const [failed, setFailed] = useState(false);
  const count = members.length;
  const title = t(setting.nonCompliantTitleKey);

  const change = (next: boolean) => {
    if (!next) setFailed(false);
    onOpenChange(next);
  };
  const remind = async () => {
    setFailed(false);
    const outcome = await send();
    if (outcome === "error") {
      setFailed(true);
      return;
    }
    change(false);
    if (outcome === "limit") onLimit();
    else toast.success(t("workspaces.twoFactorRemindSentToast"));
  };

  return (
    <Dialog open={open} onOpenChange={change}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {t("workspaces.loginMembersDescription", { count })}
          </DialogDescription>
        </DialogHeader>
        <ul
          aria-label={title}
          className="max-h-[12.5rem] min-w-0 divide-y overflow-y-auto rounded-md border text-sm"
        >
          {members.map((member) => (
            <li key={member.userId} className="truncate px-3 py-2">
              {member.email || member.name || member.userId}
            </li>
          ))}
        </ul>
        {requirement === "twoFactor" ? (
          <section
            aria-label={t("workspaces.twoFactorRemindTitle", { count })}
            className="bg-muted/40 min-w-0 space-y-2 rounded-lg border px-4 py-3 text-sm"
          >
            <p className="font-medium">
              {t("workspaces.twoFactorRemindTitle", { count })}
            </p>
            <p className="text-muted-foreground">
              {t("workspaces.twoFactorRemindBody", { count })}
            </p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={sending}
              onClick={() => void remind()}
            >
              {sending ? <Loader2 className="animate-spin" /> : null}
              {t("workspaces.twoFactorRemindAction", { count })}
            </Button>
          </section>
        ) : null}
        {failed ? (
          <p role="alert" className="text-destructive text-sm">
            {t("workspaces.twoFactorRemindError")}
          </p>
        ) : null}
        <DialogFooter>
          <Button type="button" onClick={() => change(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** Render's "Remind limit has been reached!": today's batch already went. */
function ReminderLimitDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (next: boolean) => void;
}) {
  const { t } = useTranslations();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader className="items-center text-center sm:text-center">
          <TriangleAlert
            aria-hidden="true"
            className="text-destructive size-6"
          />
          <DialogTitle>{t("workspaces.twoFactorRemindLimitTitle")}</DialogTitle>
          <DialogDescription>
            {t("workspaces.twoFactorRemindLimitBody")}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" onClick={() => onOpenChange(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
