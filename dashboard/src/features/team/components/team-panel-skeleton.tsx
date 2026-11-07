import { LogOut, Search, Trash2, UserPlus } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Skeleton } from "@/common/components/ui/skeleton";
import { TableActionsHead } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";

// Placeholder text with the shapes a member row renders: an email and a Kratos
// identity id. Rendered transparent in the same classes as MemberRow, so the
// row wraps exactly as the loaded one does on a narrow screen (w9/082: the
// bars this replaced drew a 174px card that loaded at 379px, 741px on mobile).
const PLACEHOLDER_EMAIL = "member@example.com";
const PLACEHOLDER_SUBJECT = "00000000-0000-0000-0000-000000000000";

/**
 * The Team card's content while its members load: the search box, the table
 * header and one row shaped like the caller's own (the one member every
 * workspace has — "You" and "Owner" badges, the identity id line, a role
 * select and a remove button). A workspace with more members still grows by
 * their rows on a first visit; a revisit reads them from the cache instead.
 */
export function TeamMembersSkeleton({ canManage }: { canManage: boolean }) {
  const { t } = useTranslations();
  return (
    <div className="space-y-6" aria-hidden data-testid="team-members-skeleton">
      <div className="relative max-w-sm">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="search"
          disabled
          tabIndex={-1}
          placeholder={t("team.searchPlaceholder")}
          className="pl-8"
        />
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("team.colMember")}</TableHead>
            <TableHead>{t("team.colRole")}</TableHead>
            <TableActionsHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow data-skeleton-region="member-row">
            <TableCell className="break-all">
              <div className="flex flex-wrap items-center gap-2">
                <span className="min-w-0 text-transparent">
                  {PLACEHOLDER_EMAIL}
                </span>
                <Badge variant="secondary" className="shrink-0">
                  <span className="text-transparent">{t("team.you")}</span>
                </Badge>
                <Badge variant="outline" className="shrink-0">
                  <span className="text-transparent">{t("team.owner")}</span>
                </Badge>
              </div>
              <div className="font-mono text-xs text-transparent">
                {PLACEHOLDER_SUBJECT}
              </div>
            </TableCell>
            <TableCell>
              {/* The caller can't change their own role or remove themselves,
                  so MemberRow wraps both controls in PermissionTooltip's
                  inline-flex span, whose line box is ~4px taller than the
                  control alone: the skeleton wraps them the same way. */}
              {canManage ? (
                <span className="inline-flex">
                  <Skeleton className="h-8 w-[150px]" />
                </span>
              ) : (
                <span className="text-transparent">{t("team.role.ADMIN")}</span>
              )}
            </TableCell>
            <TableCell className="text-right">
              {canManage ? (
                <span className="inline-flex">
                  <Button variant="ghost" size="icon" disabled tabIndex={-1}>
                    <Trash2 className="text-transparent" />
                  </Button>
                </span>
              ) : null}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  );
}

/**
 * The whole Team card before the workspace is known (the route and settings
 * skeletons): TeamPanel's static header — title, description and the Invite
 * button an admin sees — over TeamMembersSkeleton.
 */
export function TeamPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="team-panel-skeleton">
      <CardHeader>
        <CardTitle className="flex min-h-5 items-center gap-2">
          {t("team.title")}
        </CardTitle>
        <CardDescription>{t("team.description")}</CardDescription>
        <CardAction aria-hidden>
          <Button variant="outline" size="sm" disabled tabIndex={-1}>
            <UserPlus /> {t("team.invite")}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-6">
        <TeamMembersSkeleton canManage />
      </CardContent>
    </Card>
  );
}

/**
 * The Leave workspace card while membership loads: the ready card's static
 * header and its (disabled) button, so the danger zone neither pops in nor
 * pushes the Delete card down once the members arrive. `workspaceName` is the
 * description's real value when known; otherwise a placeholder of similar
 * length is drawn transparent.
 */
export function LeaveWorkspaceCardSkeleton({
  workspaceName,
}: {
  workspaceName?: string;
}) {
  const { t } = useTranslations();
  const description = t("team.leaveDescription", {
    workspace: workspaceName ?? "My workspace",
  });
  return (
    <Card
      className="border-destructive/50"
      data-testid="leave-workspace-skeleton"
      aria-hidden
    >
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("team.leaveTitle")}
        </CardTitle>
        <CardDescription
          className={workspaceName ? undefined : "text-transparent"}
        >
          {description}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button
          variant="outline"
          className="text-destructive"
          disabled
          tabIndex={-1}
        >
          <LogOut />
          {t("team.leaveAction")}
        </Button>
      </CardContent>
    </Card>
  );
}
