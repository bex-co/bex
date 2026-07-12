import { useState } from "react";
import { Loader2, UserPlus } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useInviteMember } from "@/features/team/hooks/use-invite-member";
import { ROLES, type Role } from "@/features/team/types";

export interface InviteMemberDialogProps {
  workspaceId: string;
  /** Called once an invite succeeds (the pending list refetches). */
  onInvited: () => void;
}

/**
 * The invite flow (w4/m12/t004): an email and a role. On success the recipient
 * is emailed and joins the workspace on their first login (docs/ADR012-auth.md); the
 * dialog closes and the pending-invite list refreshes.
 */
export function InviteMemberDialog({ workspaceId, onInvited }: InviteMemberDialogProps) {
  const { t } = useTranslations();
  const { invite, busy } = useInviteMember(workspaceId);

  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("DEVELOPER");

  async function handleSubmit() {
    if (!email.trim() || busy) return;
    const ok = await invite(email.trim(), role);
    if (ok) {
      onInvited();
      setOpen(false);
      setEmail("");
      setRole("DEVELOPER");
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <UserPlus /> {t("team.invite")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("team.inviteTitle")}</DialogTitle>
          <DialogDescription>{t("team.inviteDescription")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="invite-email">{t("team.fieldEmail")}</Label>
          <Input
            id="invite-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder={t("team.fieldEmailPlaceholder")}
            autoComplete="off"
            autoFocus
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleSubmit();
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="invite-role">{t("team.fieldRole")}</Label>
          <Select value={role} onValueChange={(value) => setRole(value as Role)}>
            <SelectTrigger id="invite-role" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLES.map((r) => (
                <SelectItem key={r} value={r}>
                  {t(`team.role.${r}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={busy}>
              {t("team.inviteCancel")}
            </Button>
          </DialogClose>
          <Button onClick={() => void handleSubmit()} disabled={!email.trim() || busy}>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("team.inviteSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
