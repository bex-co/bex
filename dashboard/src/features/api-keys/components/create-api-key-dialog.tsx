import { useId, useState } from "react";
import { Plus } from "lucide-react";
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
import { cn } from "@/common/lib/utils/utils";
import { config } from "@/config/config";
import { ApiKeyPermissionsPicker } from "@/features/api-keys/components/api-key-permissions-picker";
import {
  PICKER_DIALOG_BODY_CLASS,
  PICKER_DIALOG_CLASS,
  SCROLLING_DIALOG_CLASS,
} from "@/features/api-keys/components/picker-dialog-layout";
import { useApiKeyPermissionCatalog } from "@/features/api-keys/hooks/use-api-key-permission-catalog";
import { useCreateApiKey } from "@/features/api-keys/hooks/use-create-api-key";
import { curlExample } from "@/features/api-keys/lib/bearer-key";
import {
  ALL_SELECTION,
  selectionToInput,
} from "@/features/api-keys/lib/permissions";
import type {
  CreatedApiKey,
  PermissionSelection,
} from "@/features/api-keys/types";

export interface CreateApiKeyDialogProps {
  /** Called once a create succeeds (before the key is dismissed). */
  onCreated: () => void;
}

/**
 * The mint flow (w4/m8/t003, w4/m105; Render's shape since w7/m426): a name
 * and the key's permissions (w9/m496), then the key. The created step leads
 * with the one string the user sends as `Authorization: Bearer` and a curl
 * that works when pasted, as Render's "make your first request" step does. The
 * same credential is also an OAuth2 client, so its client ID, client secret
 * and the optional client_credentials exchange sit in a detail that starts
 * collapsed. What the key can do is stated in the primary area, never only
 * inside that detail.
 *
 * `created` is local component state only — never written to Apollo's cache
 * (the hook uses `fetchPolicy: "no-cache"`) — and is discarded on close, so
 * there is no path to re-display a minted key after the dialog dismisses. The
 * dialog is uncontrolled (no `open` state of our own): Radix already fires
 * `onOpenChange` for every close path (Done/Cancel/Escape/overlay), which is
 * all the reset needs, and the name step's own state goes with its content.
 */
export function CreateApiKeyDialog({ onCreated }: CreateApiKeyDialogProps) {
  const { t } = useTranslations();
  const [created, setCreated] = useState<CreatedApiKey | null>(null);

  return (
    <Dialog
      onOpenChange={(next) => {
        if (!next) setCreated(null);
      }}
    >
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Plus /> {t("apiKeys.create")}
        </Button>
      </DialogTrigger>
      {/* The key, the curl and the token endpoint are long unbroken strings. A
          bounded column keeps their intrinsic width from widening the grid
          past a narrow viewport; each one wraps inside its own box. */}
      <DialogContent
        className={cn(SCROLLING_DIALOG_CLASS, !created && PICKER_DIALOG_CLASS)}
      >
        {created ? (
          <CreatedKeyStep created={created} />
        ) : (
          // Mounted only while the dialog is open on this step, so a close
          // (Cancel, Escape, the overlay) discards the name and the choice.
          <CreateKeyForm
            onCreated={(key) => {
              setCreated(key);
              onCreated(); // refresh the list now; the key stays on screen independently
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

/**
 * The name step: a name, then which permissions the key gets (w9/m496). It
 * opens on All, the grant bex-api gives a key created with none named and the
 * one every key had before keys could be scoped. Until the catalog loads (or
 * if it cannot), Create still works and sends that All.
 */
function CreateKeyForm({
  onCreated,
}: {
  onCreated: (key: CreatedApiKey) => void;
}) {
  const { t } = useTranslations();
  const { create, busy } = useCreateApiKey();
  const catalog = useApiKeyPermissionCatalog();
  const [name, setName] = useState("");
  const [selection, setSelection] =
    useState<PermissionSelection>(ALL_SELECTION);

  async function handleSubmit() {
    if (!name.trim() || busy) return;
    const key = await create(
      name.trim(),
      selectionToInput(selection, catalog.areas ?? []),
    );
    if (key) onCreated(key);
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("apiKeys.createTitle")}</DialogTitle>
        <DialogDescription>{t("apiKeys.createDescription")}</DialogDescription>
      </DialogHeader>
      <div className={cn(PICKER_DIALOG_BODY_CLASS, "space-y-6")}>
        <div className="space-y-2">
          <Label htmlFor="api-key-name">{t("apiKeys.fieldName")}</Label>
          <Input
            id="api-key-name"
            maxLength={100}
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
        <ApiKeyPermissionsPicker
          areas={catalog.areas}
          error={catalog.error}
          errorMessage={t("apiKeys.permCatalogErrorCreate")}
          refetch={catalog.refetch}
          value={selection}
          onChange={setSelection}
          disabled={busy}
        />
      </div>
      <DialogFooter>
        <DialogClose asChild>
          <Button variant="outline" disabled={busy}>
            {t("apiKeys.createCancel")}
          </Button>
        </DialogClose>
        {/* `loading` draws the spinner over the label instead of beside
            it, so the button is the same width idle and busy. */}
        <Button
          onClick={() => void handleSubmit()}
          disabled={!name.trim()}
          loading={busy}
        >
          {t("apiKeys.createSubmit")}
        </Button>
      </DialogFooter>
    </>
  );
}

/** The one-time display of a minted key. */
function CreatedKeyStep({ created }: { created: CreatedApiKey }) {
  const { t } = useTranslations();
  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("apiKeys.createdTitle")}</DialogTitle>
        <DialogDescription>{t("apiKeys.createdWarning")}</DialogDescription>
      </DialogHeader>
      <div className="-mx-6 -my-1 min-h-0 space-y-4 overflow-y-auto px-6 py-1">
        <div className="space-y-2">
          <CredentialField
            label={t("apiKeys.fieldKey")}
            value={created.key}
            copyLabel={t("apiKeys.copyKey")}
          />
          <p className="text-muted-foreground text-sm">
            {t("apiKeys.keyHelp")}
          </p>
        </div>
        <CredentialField
          block
          label={t("apiKeys.curlTitle")}
          value={curlExample(config.apiBaseUrl, created.key)}
          copyLabel={t("apiKeys.copyCurl")}
        />
        <details className="rounded-md border">
          <summary className="cursor-pointer rounded-md px-3 py-2 text-sm font-medium">
            {t("apiKeys.oauthDetails")}
          </summary>
          <div className="space-y-3 border-t p-3">
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
            <p className="text-muted-foreground text-sm [overflow-wrap:anywhere] whitespace-pre-line">
              {t("apiKeys.exchangeHelp", {
                tokenEndpoint: config.oauthTokenEndpoint,
              })}
            </p>
          </div>
        </details>
      </div>
      <DialogFooter>
        <DialogClose asChild>
          <Button>{t("apiKeys.createdDone")}</Button>
        </DialogClose>
      </DialogFooter>
    </>
  );
}

/**
 * One labelled value with its copy button. `block` is a multi-line code sample
 * (the curl): it keeps its line breaks and indentation, and its copy button
 * moves up beside the label so the sample has the full width. Either way the
 * value wraps anywhere inside its box, so a 67-character key or a long URL
 * never widens the dialog.
 */
function CredentialField({
  label,
  value,
  copyLabel,
  block = false,
}: {
  label: string;
  value: string;
  copyLabel: string;
  block?: boolean;
}) {
  const { t } = useTranslations();
  const labelId = useId();
  const copy = (
    <CopyButton
      value={value}
      label={copyLabel}
      successText={t("apiKeys.copied")}
      errorText={t("apiKeys.copyError")}
    />
  );
  return (
    <div role="group" aria-labelledby={labelId} className="space-y-1.5">
      {block ? (
        <>
          <div className="flex items-center justify-between gap-2">
            <Label id={labelId}>{label}</Label>
            {copy}
          </div>
          <pre className="rounded-md border bg-muted/50 p-3 font-mono text-xs break-all whitespace-pre-wrap">
            {value}
          </pre>
        </>
      ) : (
        <>
          <Label id={labelId}>{label}</Label>
          <div className="flex items-center gap-2 rounded-md border bg-muted/50 p-3">
            <code className="min-w-0 flex-1 overflow-x-auto font-mono text-sm break-all">
              {value}
            </code>
            {copy}
          </div>
        </>
      )}
    </div>
  );
}
