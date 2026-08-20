import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ArrowUpRight, Loader2, UserPlus } from "lucide-react";
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
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/common/components/ui/alert";
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
import { isValidEmail } from "@/common/lib/utils/email";
import { useInviteMember } from "@/features/team/hooks/use-invite-member";
import { ROLES, type Role } from "@/features/team/types";

export interface InviteMemberDialogProps {
  workspaceId: string;
  /** Seats exhausted on a limited plan (w1/m33): the dialog opens straight to
   *  the plan wall with submit disabled — the caller sees the cap before
   *  composing an invite the server would refuse. */
  seatsFull?: boolean;
  /** Called once an invite succeeds (the pending list refetches). */
  onInvited: () => void;
}

/**
 * The invite flow (w4/m12/t004): an email and a role. On success the recipient
 * is emailed and joins the workspace on their first login (docs/ADR012-auth.md); the
 * dialog closes and the pending-invite list refreshes.
 *
 * A plan refusal (the Hobby member cap, or a role the plan doesn't offer) stays
 * inline with an upgrade CTA that opens the change-plan dialog on workspace
 * settings (w6/m15/t001) — the rejection used to be a toast with nowhere to go.
 */
export function InviteMemberDialog({
  workspaceId,
  seatsFull = false,
  onInvited,
}: InviteMemberDialogProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { invite, busy, refusal, clearRefusal, planLimit } =
    useInviteMember(workspaceId);
  const planWall = planLimit ?? (seatsFull ? t("team.seatsFullBody") : null);

  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("DEVELOPER");

  // Gate submit on a plausible address so an obviously-malformed one is caught
  // here rather than after a round trip (the server stays authoritative).
  const emailValid = isValidEmail(email);
  const showEmailError = email.trim() !== "" && !emailValid;

  async function handleSubmit() {
    if (!emailValid || busy) return;
    const ok = await invite(email.trim(), role);
    if (ok) {
      onInvited();
      setOpen(false);
      setEmail("");
      setRole("DEVELOPER");
    }
  }

  // The CTA leaves the invite behind, so close the dialog rather than stranding
  // it open behind the settings page. `plan=change` is what opens the plan
  // dialog on arrival (routes/workspace.settings.tsx).
  function handleUpgrade() {
    setOpen(false);
    void navigate({ to: "/workspace/settings", search: { plan: "change" } });
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
            onChange={(e) => {
              setEmail(e.target.value);
              clearRefusal();
            }}
            placeholder={t("team.fieldEmailPlaceholder")}
            autoComplete="off"
            autoFocus
            aria-invalid={showEmailError || !!refusal}
            aria-describedby={
              showEmailError
                ? "invite-email-invalid"
                : refusal
                  ? "invite-email-refusal"
                  : undefined
            }
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleSubmit();
            }}
          />
          {showEmailError ? (
            <p id="invite-email-invalid" className="text-destructive text-sm">
              {t("team.fieldEmailInvalid")}
            </p>
          ) : refusal ? (
            <p
              id="invite-email-refusal"
              role="alert"
              className="text-destructive text-sm"
            >
              {refusal}
            </p>
          ) : null}
        </div>
        <div className="space-y-2">
          <Label htmlFor="invite-role">{t("team.fieldRole")}</Label>
          <Select
            value={role}
            onValueChange={(value) => setRole(value as Role)}
          >
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
        {planWall ? (
          <Alert variant="destructive">
            <AlertTitle>{t("team.inviteErrorPlanTitle")}</AlertTitle>
            <AlertDescription className="flex flex-col items-start gap-2">
              <span>{planWall}</span>
              <Button
                variant="link"
                className="h-auto p-0"
                onClick={handleUpgrade}
              >
                {t("team.inviteErrorPlanCta")}
                <ArrowUpRight />
              </Button>
            </AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={busy}>
              {t("team.inviteCancel")}
            </Button>
          </DialogClose>
          <Button
            onClick={() => void handleSubmit()}
            disabled={!emailValid || busy || seatsFull}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("team.inviteSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
