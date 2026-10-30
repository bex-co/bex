import { Link } from "@tanstack/react-router";
import { CheckCircle2, Circle, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useGitConnections } from "@/features/git/hooks/use-git-connection";
import { useTeam } from "@/features/team/hooks/use-team";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useGettingStartedDismissal } from "../hooks/use-getting-started-dismissal";

interface ChecklistItem {
  key: string;
  done: boolean;
  to: string;
  hash?: string;
  search?: Record<string, string>;
}

/**
 * ADR075 D2 getting-started checklist for a workspace with no resources. Its
 * host renders it only in that state; creating the first resource replaces it
 * with the resource list. Verify-email and add-a-card items are deliberately
 * absent: both are hard gates before the dashboard (w2/m168, w6/m42), so a
 * user who can see this card has already done them.
 */
/**
 * A Hobby workspace has one seat, so inviting dead-ends in "Your plan can't
 * take this invite" (as on Render). While no seat is free, the item offers the
 * plan change that makes inviting possible instead.
 */
function inviteItem(team: ReturnType<typeof useTeam>): ChecklistItem {
  const done = team.members.length > 1 || team.invites.length > 0;
  const seatsFull =
    team.seats != null &&
    team.seats.limit > 0 &&
    team.seats.used >= team.seats.limit;
  if (seatsFull && !done) {
    return {
      key: "upgradeToInvite",
      done: false,
      to: "/workspace/settings",
      search: { plan: "change" },
    };
  }
  return { key: "inviteTeammate", done, to: "/workspace/settings" };
}

export function GettingStartedCard() {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const { dismissed, dismiss } = useGettingStartedDismissal(currentWorkspaceId);
  const git = useGitConnections();
  const team = useTeam(currentWorkspaceId);

  if (dismissed) return null;

  const items: ChecklistItem[] = [
    { key: "createService", done: false, to: "/services/new" },
    {
      key: "connectGithub",
      done: git.connected,
      to: "/settings",
      hash: "integrations",
    },
    inviteItem(team),
  ];

  return (
    <Card data-testid="getting-started-card">
      <CardHeader className="flex flex-row items-start justify-between gap-2">
        <div className="space-y-1">
          <CardTitle>{t("onboarding.gettingStartedTitle")}</CardTitle>
          <p className="text-sm text-muted-foreground">
            {t("onboarding.gettingStartedSubtitle")}
          </p>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t("onboarding.gettingStartedDismiss")}
          onClick={dismiss}
        >
          <X className="size-4" />
        </Button>
      </CardHeader>
      <CardContent>
        <ul className="space-y-2">
          {items.map((item) => (
            <li key={item.key} className="flex items-center gap-3">
              {item.done ? (
                <CheckCircle2
                  className="size-5 shrink-0 text-emerald-600"
                  aria-label={t("onboarding.gettingStartedDone")}
                />
              ) : (
                <Circle
                  className="size-5 shrink-0 text-muted-foreground"
                  aria-label={t("onboarding.gettingStartedPending")}
                />
              )}
              {item.done ? (
                <span className="text-sm text-muted-foreground line-through">
                  {t(`onboarding.gettingStarted.${item.key}`)}
                </span>
              ) : (
                <Link
                  to={item.to}
                  hash={item.hash}
                  search={item.search}
                  className="text-sm font-medium underline-offset-4 hover:underline"
                >
                  {t(`onboarding.gettingStarted.${item.key}`)}
                </Link>
              )}
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
