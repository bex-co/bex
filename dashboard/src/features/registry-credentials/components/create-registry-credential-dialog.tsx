import { useState } from "react";
import { Loader2, Plus } from "lucide-react";
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
import { LabelError } from "@/common/components/label-error";
import { isInvalidLabel } from "@/common/lib/utils/resource-label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCreateRegistryCredential } from "@/features/registry-credentials/hooks/use-create-registry-credential";
import { CredentialFieldError } from "@/features/registry-credentials/components/credential-field-error";
import {
  authTokenError,
  registryHostError,
  usernameError,
} from "@/features/registry-credentials/lib/credential-fields";

export interface CreateRegistryCredentialDialogProps {
  /** Called once a create succeeds (before the dialog is dismissed). */
  onCreated: () => void;
}

/**
 * The add-credential flow (w2/m14/t006): host + username + secret, with an
 * optional display name. Unlike the API-key mint dialog, the secret here is
 * something the caller already typed in (a registry password/token they own)
 * — there's nothing to "reveal" back, so this is a single-step form, not a
 * form-then-reveal two-step dialog.
 */
export function CreateRegistryCredentialDialog({
  onCreated,
}: CreateRegistryCredentialDialogProps) {
  const { t } = useTranslations();
  const { create, busy } = useCreateRegistryCredential();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [username, setUsername] = useState("");
  const [authToken, setAuthToken] = useState("");

  // The fields the server refuses are marked in place and hold Add (w21/053).
  const hostReason = registryHostError(host);
  const usernameReason = usernameError(username);
  const tokenReason = authTokenError(authToken);
  // The name is optional (it defaults to the host) but, when given, a label.
  const canSubmit =
    host.trim() &&
    username.trim() &&
    authToken.trim() &&
    !isInvalidLabel(name) &&
    !hostReason &&
    !usernameReason &&
    !tokenReason;

  function reset() {
    setName("");
    setHost("");
    setUsername("");
    setAuthToken("");
  }

  async function handleSubmit() {
    if (!canSubmit || busy) return;
    const ok = await create({
      host: host.trim(),
      username: username.trim(),
      authToken: authToken.trim(),
      name: name.trim(),
    });
    if (ok) {
      onCreated();
      reset();
      setOpen(false);
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
          <Plus /> {t("registryCredentials.create")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("registryCredentials.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("registryCredentials.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="rc-host">
              {t("registryCredentials.fieldHost")}
            </Label>
            <Input
              id="rc-host"
              value={host}
              onChange={(e) => setHost(e.target.value)}
              placeholder={t("registryCredentials.fieldHostPlaceholder")}
              autoComplete="off"
              autoFocus
              aria-invalid={hostReason !== null}
              aria-describedby={hostReason ? "rc-host-error" : undefined}
              className="font-mono text-sm"
            />
            <CredentialFieldError id="rc-host-error" reason={hostReason} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rc-username">
              {t("registryCredentials.fieldUsername")}
            </Label>
            <Input
              id="rc-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="off"
              aria-invalid={usernameReason !== null}
              aria-describedby={
                usernameReason ? "rc-username-error" : undefined
              }
              className="font-mono text-sm"
            />
            <CredentialFieldError
              id="rc-username-error"
              reason={usernameReason}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rc-auth-token">
              {t("registryCredentials.fieldAuthToken")}
            </Label>
            <Input
              id="rc-auth-token"
              type="password"
              value={authToken}
              onChange={(e) => setAuthToken(e.target.value)}
              autoComplete="new-password"
              aria-invalid={tokenReason !== null}
              aria-describedby={tokenReason ? "rc-auth-token-error" : undefined}
              className="font-mono text-sm"
              onKeyDown={(e) => {
                if (e.key === "Enter" && canSubmit) void handleSubmit();
              }}
            />
            <CredentialFieldError
              id="rc-auth-token-error"
              reason={tokenReason}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rc-name">
              {t("registryCredentials.fieldName")}
              <span className="text-muted-foreground ml-1 font-normal">
                {t("registryCredentials.fieldNameOptional")}
              </span>
            </Label>
            <Input
              id="rc-name"
              aria-invalid={isInvalidLabel(name)}
              aria-describedby={
                isInvalidLabel(name) ? "rc-name-error" : undefined
              }
              maxLength={100}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={
                host || t("registryCredentials.fieldHostPlaceholder")
              }
            />
            <LabelError id="rc-name-error" value={name} />
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={busy}>
              {t("registryCredentials.createCancel")}
            </Button>
          </DialogClose>
          <Button
            onClick={() => void handleSubmit()}
            disabled={!canSubmit || busy}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("registryCredentials.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
