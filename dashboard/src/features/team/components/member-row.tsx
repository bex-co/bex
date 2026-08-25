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
import { useTranslations } from "@/common/hooks/use-translations";
import { ROLES, type MemberView, type Role } from "@/features/team/types";

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

  return (
    <TableRow>
      <TableCell className="break-all">
        <div className="flex items-center gap-2">
          <span>{identity}</span>
          {member.mfaEnabled ? (
            <Badge
              variant="outline"
              className="gap-1 text-muted-foreground"
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
          <Select
            value={member.role}
            disabled={changing}
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
        ) : (
          <span>{t(`team.role.${member.role}`)}</span>
        )}
      </TableCell>
      <TableCell className="text-right">
        {canManage ? (
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
            description={t("team.removeConfirm", { identity })}
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
