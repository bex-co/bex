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
import { CopyButton } from "@/common/components/copy-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { config } from "@/config/config";
import { useCreateApiKey } from "@/features/api-keys/hooks/use-create-api-key";
import type { CreatedApiKey } from "@/features/api-keys/types";

export interface CreateApiKeyDialogProps {
  /** Called once a create succeeds (before the secret dialog is dismissed). */
  onCreated: () => void;
}

/**
 * The mint flow (w4/m8/t003, extended w4/m105): a name, then both the
 * `client_id` and the one-time secret with copy affordances, plus the
 * client_credentials exchange the credential actually needs. `created` is
 * local component state only — never written to Apollo's cache (the hook
 * uses `fetchPolicy: "no-cache"`) — and is discarded on close, so there is no
 * path to re-display a minted secret after the dialog dismisses. The dialog is
 * uncontrolled (no `open` state of our own): Radix already fires
 * `onOpenChange` for every close path (Done/Cancel/Escape/overlay), which is
 * all `reset()` needs.
 */
export function CreateApiKeyDialog({ onCreated }: CreateApiKeyDialogProps) {
  const { t } = useTranslations();
  const { create, busy } = useCreateApiKey();

  const [name, setName] = useState("");
  const [created, setCreated] = useState<CreatedApiKey | null>(null);

  async function handleSubmit() {
    if (!name.trim() || busy) return;
    const key = await create(name.trim());
    if (key) {
      setCreated(key);
      onCreated(); // refresh the list now; the secret dialog stays open independently
    }
  }

  return (
    <Dialog
      onOpenChange={(next) => {
        if (!next) {
          setName("");
          setCreated(null);
        }
      }}
    >
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Plus /> {t("apiKeys.create")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        {created ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("apiKeys.createdTitle")}</DialogTitle>
              <DialogDescription>
                {t("apiKeys.createdWarning")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <CredentialField
                label={t("apiKeys.fieldClientId")}
                value={created.id}
                copyLabel={t("apiKeys.copyClientId")}
              />
              <CredentialField
                label={t("apiKeys.fieldSecret")}
                value={created.secret}
                copyLabel={t("apiKeys.copySecret")}
              />
              <p className="text-muted-foreground text-sm whitespace-pre-line">
                {t("apiKeys.exchangeHelp", {
                  tokenEndpoint: config.oauthTokenEndpoint,
                })}
              </p>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button>{t("apiKeys.createdDone")}</Button>
              </DialogClose>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("apiKeys.createTitle")}</DialogTitle>
              <DialogDescription>
                {t("apiKeys.createDescription")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              <Label htmlFor="api-key-name">{t("apiKeys.fieldName")}</Label>
              <Input
                id="api-key-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("apiKeys.fieldNamePlaceholder")}
                autoComplete="off"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter") void handleSubmit();
                }}
              />
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline" disabled={busy}>
                  {t("apiKeys.createCancel")}
                </Button>
              </DialogClose>
              <Button
                onClick={() => void handleSubmit()}
                disabled={!name.trim() || busy}
              >
                {busy ? <Loader2 className="animate-spin" /> : null}
                {t("apiKeys.createSubmit")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function CredentialField({
  label,
  value,
  copyLabel,
}: {
  label: string;
  value: string;
  copyLabel: string;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      <div className="flex items-center gap-2 rounded-md border bg-muted/50 p-3">
        <code className="flex-1 overflow-x-auto font-mono text-sm break-all">
          {value}
        </code>
        <CopyButton
          value={value}
          label={copyLabel}
          successText={t("apiKeys.copied")}
          errorText={t("apiKeys.copyError")}
        />
      </div>
    </div>
  );
}
