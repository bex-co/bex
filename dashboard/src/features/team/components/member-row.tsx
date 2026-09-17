import { useState } from "react";
import { Loader2, ShieldCheck, Trash2 } from "lucide-react";
import { TableCell, TableRow } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { ROLES, type MemberView, type Role } from "@/features/team/types";
import { memberActionReasonKeys } from "@/features/team/lib/member-action-reasons";

export interface MemberRowProps {
  member: MemberView;
  /** Whether the caller may manage members (admin) — read-only rows otherwise. */
  canManage: boolean;
  changing: boolean;
  removing: boolean;
  onChangeRole: (subject: string, role: Role) => void;
  onRemove: (subject: string) => void;
}

/**
 * One accepted-member row: the member's email as primary identity (falling
 * back to their opaque userId when email is unresolvable — never blank), a
 * role dropdown (admin only), and a remove action gated behind a
 * confirmation. The raw subject stays the mutation key but is demoted to a
 * muted secondary line (w6/m10).
 *
 * Two rows carry membership invariants the server enforces (w5/m101): the
 * workspace owner cannot be removed or demoted, and nobody may act on their
 * own membership. Their controls stay rendered but disabled with the reason
 * (the m50 always-rendered-disabled pattern) so the surface never offers an
 * action the API refuses — the inline server error remains the backstop for
 * the race where someone else changes roles mid-session.
 */
export function MemberRow({
  member,
  canManage,
  changing,
  removing,
  onChangeRole,
  onRemove,
}: MemberRowProps) {
  const { t } = useTranslations();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const identity = member.email || member.userId || member.subject;
  const reasonKeys = memberActionReasonKeys(member);
  const removeReason = reasonKeys.removeReason
    ? t(reasonKeys.removeReason)
    : null;
  const roleReason = reasonKeys.roleReason ? t(reasonKeys.roleReason) : null;

  return (
    <TableRow>
      <TableCell className="break-all">
        {/* flex-wrap + shrink-0 badges: the cell is break-all, so a badge
            competing for width on a narrow viewport would otherwise squeeze the
            email down to its min-content and break it one character per line.
            Wrapping lets the badges drop to their own line instead. */}
        <div className="flex flex-wrap items-center gap-2">
          <span className="min-w-0">{identity}</span>
          {member.isSelf ? (
            <Badge variant="secondary" className="shrink-0">
              {t("team.you")}
            </Badge>
          ) : null}
          {member.isOwner ? (
            <Badge
              variant="outline"
              className="shrink-0"
              title={t("team.ownerTooltip")}
            >
              {t("team.owner")}
            </Badge>
          ) : null}
          {!member.identityResolved ? (
            <Badge
              variant="outline"
              className="shrink-0 text-muted-foreground"
              title={t("team.identityUnresolvedTooltip")}
            >
              {t("team.identityUnresolved")}
            </Badge>
          ) : null}
          {member.mfaEnabled ? (
            <Badge
              variant="outline"
              className="shrink-0 gap-1 text-muted-foreground"
              title={t("team.mfaEnabledTooltip")}
            >
              <ShieldCheck className="size-3" aria-hidden />
              {t("team.mfaEnabled")}
            </Badge>
          ) : null}
        </div>
        {identity !== member.subject ? (
          <div className="font-mono text-xs text-muted-foreground">
            {member.subject}
          </div>
        ) : null}
      </TableCell>
      <TableCell>
        {canManage ? (
          <PermissionTooltip reason={roleReason}>
            <Select
              value={member.role}
              disabled={changing || roleReason !== null}
              onValueChange={(value) =>
                onChangeRole(member.subject, value as Role)
              }
            >
              <SelectTrigger size="sm" className="w-[150px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLES.map((role) => (
                  <SelectItem key={role} value={role}>
                    {t(`team.role.${role}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </PermissionTooltip>
        ) : (
          <span>{t(`team.role.${member.role}`)}</span>
        )}
      </TableCell>
      <TableCell className="text-right">
        {canManage && removeReason ? (
          // Rendered, disabled, and explained — not hidden: the control's
          // absence would read as "this row has no actions", when the truth is
          // "this action is refused, and here is why".
          <PermissionTooltip reason={removeReason}>
            <Button
              variant="ghost"
              size="icon"
              disabled
              aria-label={t("team.remove")}
            >
              <Trash2 className="text-muted-foreground" />
            </Button>
          </PermissionTooltip>
        ) : canManage ? (
          <ConfirmDialog
            open={confirmOpen}
            onOpenChange={setConfirmOpen}
            trigger={
              <Button
                variant="ghost"
                size="icon"
                disabled={removing}
                aria-label={t("team.remove")}
              >
                {removing ? (
                  <Loader2 className="animate-spin" />
                ) : (
                  <Trash2 className="text-destructive" />
                )}
              </Button>
            }
            title={t("team.removeTitle")}
            // Two sentences, deliberately: losing access is expected, but
            // revoking the member's API keys is a destructive side effect that
            // can break automation (w2/m163). An admin has to see that cost
            // before confirming, not discover it as an outage afterwards.
            description={`${t("team.removeConfirm", { identity })} ${t(
              "team.removeRevokesKeys",
              { identity },
            )}`}
            cancelLabel={t("team.removeCancel")}
            confirmLabel={t("team.remove")}
            onConfirm={() => {
              setConfirmOpen(false);
              onRemove(member.subject);
            }}
          />
        ) : null}
      </TableCell>
    </TableRow>
  );
}
