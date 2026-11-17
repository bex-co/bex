import { useId, useState } from "react";
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
import { isValidEmail } from "@/common/lib/utils/email";
import { useInviteOrganizationMember } from "@/features/organizations/hooks/use-organization-member-actions";
import {
  ORGANIZATION_INVITE_ROLES,
  ORGANIZATION_ROLE_LABEL_KEYS,
  type OrganizationRole,
} from "@/features/organizations/types";

/**
 * An Owner invites someone to the org by email, as Owner or Member (ADR115 D4:
 * bex has no SSO, so this is how a person enters the org besides a workspace
 * invite). A Guest is made by changing a person's role, not by invite.
 */
export function InviteOrganizationMemberDialog({
  organizationId,
}: {
  organizationId: string;
}) {
  const { t } = useTranslations();
  const emailId = useId();
  const roleId = useId();
  const messageId = useId();
  const { invite, busy, refusal, clearRefusal } =
    useInviteOrganizationMember(organizationId);
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<OrganizationRole>("MEMBER");
  const emailValid = isValidEmail(email);
  const emailInvalid = email.trim() !== "" && !emailValid;
  const message = emailInvalid ? t("organizations.fieldEmailInvalid") : refusal;

  function reset() {
    setEmail("");
    setRole("MEMBER");
    clearRefusal();
  }

  async function submit() {
    if (!emailValid || busy) return;
    if (await invite(email.trim(), role)) {
      setOpen(false);
      reset();
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) reset();
      }}
    >
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <UserPlus />
          {t("organizations.invite")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("organizations.inviteTitle")}</DialogTitle>
          <DialogDescription>
            {t("organizations.inviteDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor={emailId}>{t("organizations.fieldEmail")}</Label>
            <Input
              id={emailId}
              type="email"
              value={email}
              autoComplete="off"
              autoFocus
              aria-invalid={message ? true : undefined}
              aria-describedby={message ? messageId : undefined}
              onChange={(event) => {
                setEmail(event.target.value);
                clearRefusal();
              }}
            />
            {message ? (
              <p
                id={messageId}
                role={emailInvalid ? undefined : "alert"}
                className="text-sm text-destructive"
              >
                {message}
              </p>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor={roleId}>{t("organizations.fieldRole")}</Label>
            <Select
              value={role}
              onValueChange={(value) => {
                setRole(value as OrganizationRole);
                clearRefusal();
              }}
            >
              <SelectTrigger id={roleId} className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ORGANIZATION_INVITE_ROLES.map((option) => (
                  <SelectItem key={option} value={option}>
                    {t(ORGANIZATION_ROLE_LABEL_KEYS[option])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline" disabled={busy}>
                {t("organizations.cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" disabled={!emailValid || busy}>
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("organizations.inviteSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
