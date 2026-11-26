import { useId, useState, type ReactNode } from "react";
import { Loader2, Trash2 } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import { PanelTableHeader } from "@/common/components/panel-states";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";

/*
 * The pieces a member table is built from, shared by a workspace's Team card
 * and an organization's Members card (w7/m464), which differ only in their
 * roles and copy.
 */

/**
 * A member's identity cell content: the full identity, wrapping at its own
 * break points (an email's hyphen or dot), since a phone has no hover to
 * reveal a truncation, then the "You" badge on the caller's own row and any
 * further badges (`children`), which wrap to their own line when they don't
 * fit beside the name.
 */
export function MemberIdentity({
  identity,
  isSelf,
  avatar,
  children,
}: {
  identity: string;
  isSelf: boolean;
  /** The member's avatar before their name, as Render's members list draws
   *  it (w1/m648). */
  avatar?: ReactNode;
  children?: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      {avatar}
      <span className="min-w-0 max-w-full break-words">{identity}</span>
      {isSelf ? (
        <Badge variant="secondary" className="shrink-0">
          {t("team.you")}
        </Badge>
      ) : null}
      {children}
    </div>
  );
}

/** A member row's role dropdown over `roles`, each named by `label`. */
export function RoleSelect<R extends string>({
  value,
  roles,
  label,
  ariaLabel,
  disabled,
  onChange,
}: {
  value: R;
  roles: readonly R[];
  label: (role: R) => string;
  ariaLabel: string;
  disabled?: boolean;
  onChange: (role: R) => void;
}) {
  return (
    <Select
      value={value}
      disabled={disabled}
      onValueChange={(next) => onChange(next as R)}
    >
      <SelectTrigger
        size="sm"
        className="w-[7.5rem] @md:w-[150px]"
        aria-label={ariaLabel}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {roles.map((role) => (
          <SelectItem key={role} value={role}>
            {label(role)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * A member row's remove action: the icon button (a spinner while `busy`) that
 * opens the confirmation, and calls `onConfirm` once it is confirmed.
 */
export function RemoveMemberButton({
  ariaLabel,
  busy,
  title,
  description,
  cancelLabel,
  confirmLabel,
  onConfirm,
}: {
  ariaLabel: string;
  busy: boolean;
  title: string;
  description: string;
  cancelLabel: string;
  confirmLabel: string;
  onConfirm: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={setOpen}
      trigger={
        <Button
          variant="ghost"
          size="icon"
          disabled={busy}
          aria-label={ariaLabel}
        >
          {busy ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Trash2 className="text-destructive" />
          )}
        </Button>
      }
      title={title}
      description={description}
      cancelLabel={cancelLabel}
      confirmLabel={confirmLabel}
      onConfirm={() => {
        setOpen(false);
        onConfirm();
      }}
    />
  );
}

/**
 * The pending-invite list under a member table: each invite's email, its role
 * as a badge, and its row actions. `actions` receives the id of the email's
 * element, for a button the email describes.
 */
export function PendingInvitesTable<I extends { id: string; email: string }>({
  title,
  columns,
  invites,
  roleLabel,
  actions,
}: {
  title: string;
  /** The email and role column labels. */
  columns: [string, string];
  invites: readonly I[];
  roleLabel: (invite: I) => string;
  actions: (invite: I, emailId: string) => ReactNode;
}) {
  const idPrefix = useId();
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-muted-foreground">{title}</h3>
      <Table>
        <PanelTableHeader columns={columns} />
        <TableBody>
          {invites.map((invite) => {
            const emailId = `${idPrefix}-${invite.id}`;
            return (
              <TableRow key={invite.id}>
                <TableCell className="w-full max-w-0">
                  <span id={emailId} className="block break-words">
                    {invite.email}
                  </span>
                </TableCell>
                <TableCell>
                  <Badge variant="secondary">{roleLabel(invite)}</Badge>
                </TableCell>
                <TableCell className="text-right">
                  {actions(invite, emailId)}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
