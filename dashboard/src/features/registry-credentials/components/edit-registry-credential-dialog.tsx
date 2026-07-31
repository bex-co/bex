import { useState } from "react";
import { Loader2, Pencil } from "lucide-react";
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
import { useTranslations } from "@/common/hooks/use-translations";
import { useUpdateRegistryCredential } from "@/features/registry-credentials/hooks/use-update-registry-credential";
import { useRegistryCredential } from "@/features/registry-credentials/hooks/use-registry-credential";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

export interface EditRegistryCredentialDialogProps {
  entry: RegistryCredentialView;
  /** Called once an update succeeds (before the dialog is dismissed). */
  onUpdated: () => void;
}

/**
 * Edit / rotate an existing registry credential (w5/m60): rename, change the
 * username, or rotate the token. The editable fields prefill from the dedicated
 * `registryCredential` detail read; host is immutable so it's shown read-only.
 * The token field is always blank with "leave blank to keep" semantics — the
 * stored secret is never returned by the server, so it can never be echoed here.
 */
export function EditRegistryCredentialDialog({
  entry,
  onUpdated,
}: EditRegistryCredentialDialogProps) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          size="icon"
          variant="ghost"
          aria-label={t("registryCredentials.edit")}
        >
          <Pencil />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("registryCredentials.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("registryCredentials.editDescription")}
          </DialogDescription>
        </DialogHeader>
        {open ? (
          <EditFormLoader
            entry={entry}
            onSaved={() => {
              onUpdated();
              setOpen(false);
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

/**
 * Fetches the detail read and mounts the form — only rendered while the dialog
 * is open, so the `registryCredential` query never runs (and never needs an
 * Apollo client) for the closed dialogs sitting in every list row. The detail
 * resolves from the normalized cache, so the seed is usually synchronous; the
 * form is keyed on whether it has arrived, re-seeding once without an effect.
 */
function EditFormLoader({
  entry,
  onSaved,
}: {
  entry: RegistryCredentialView;
  onSaved: () => void;
}) {
  const { credential } = useRegistryCredential(entry.id);
  const seed = credential ?? entry;
  return (
    <EditForm key={credential ? "detail" : "entry"} seed={seed} onSaved={onSaved} />
  );
}

/**
 * The editable body, mounted fresh (seeded via lazy initial state) each time the
 * dialog opens and re-seeded once the detail read resolves — so no effect ever
 * writes state. host is immutable; a blank token keeps the stored secret.
 */
function EditForm({
  seed,
  onSaved,
}: {
  seed: RegistryCredentialView;
  onSaved: () => void;
}) {
  const { t } = useTranslations();
  const { update, busy } = useUpdateRegistryCredential();
  const [name, setName] = useState(seed.name);
  const [username, setUsername] = useState(seed.username);
  const [authToken, setAuthToken] = useState("");

  const canSubmit = name.trim() && username.trim();

  async function handleSubmit() {
    if (!canSubmit || busy) return;
    const ok = await update({
      id: seed.id,
      name: name.trim(),
      username: username.trim(),
      // Blank token => keep the stored secret (never rotated, never echoed).
      authToken: authToken.trim() || undefined,
    });
    if (ok) onSaved();
  }

  return (
    <>
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="rc-edit-host">
            {t("registryCredentials.fieldHost")}
          </Label>
          <Input
            id="rc-edit-host"
            value={seed.host}
            disabled
            className="font-mono text-sm"
          />
          <p className="text-muted-foreground text-xs">
            {t("registryCredentials.fieldHostImmutable")}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="rc-edit-username">
            {t("registryCredentials.fieldUsername")}
          </Label>
          <Input
            id="rc-edit-username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="off"
            className="font-mono text-sm"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="rc-edit-auth-token">
            {t("registryCredentials.fieldAuthToken")}
          </Label>
          <Input
            id="rc-edit-auth-token"
            type="password"
            value={authToken}
            onChange={(e) => setAuthToken(e.target.value)}
            autoComplete="new-password"
            placeholder={t("registryCredentials.fieldAuthTokenKeep")}
            className="font-mono text-sm"
          />
          <p className="text-muted-foreground text-xs">
            {t("registryCredentials.fieldAuthTokenKeepHint")}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="rc-edit-name">
            {t("registryCredentials.fieldName")}
          </Label>
          <Input
            id="rc-edit-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
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
          {t("registryCredentials.editSubmit")}
        </Button>
      </DialogFooter>
    </>
  );
}
