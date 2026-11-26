import { useId, useRef, useState } from "react";
import { CheckCircle2, MoreHorizontal, Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { TableCell, TableRow } from "@/common/components/ui/table";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { CopyButton } from "@/common/components/copy-button";
import { RelativeAge } from "@/common/components/relative-time";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  ORGANIZATION_KEYS_EMPTY_CLASS,
  OrganizationApiKeysFrame,
  OrganizationApiKeysTable,
  OrganizationAuditLogKeysSection,
  OrganizationKeysNoPermission,
} from "@/features/organizations/components/organization-settings-frame";
import {
  isKeyLimitRefusal,
  useOrganizationApiTokens,
  useProvisionOrganizationApiToken,
  useRevokeOrganizationApiToken,
  type OrganizationApiToken,
} from "@/features/organizations/hooks/use-organization-api-tokens";

/** Render's name field takes at most 32 characters. */
const LABEL_MAX = 32;

/**
 * Org Settings → Audit Logs → Audit Log API Keys (w1/m657), Render's section:
 * an Owner creates keys with read-only access to the audit logs of the org and
 * every workspace in it, sees them listed (Name, Key, Created, Last used) and
 * revokes them. Anyone else sees the section with Create API key disabled and
 * Render's no-permission text, and asks bex-api nothing.
 */
export function OrganizationAuditLogKeys({
  organizationId,
  canManage,
}: {
  organizationId: string;
  canManage: boolean;
}) {
  const { t } = useTranslations();
  const { tokens, loading, failed, refetch } = useOrganizationApiTokens(
    organizationId,
    canManage,
  );
  const [revoking, setRevoking] = useState<OrganizationApiToken | null>(null);

  return (
    <OrganizationAuditLogKeysSection
      action={
        <CreateOrganizationApiKeyDialog
          organizationId={organizationId}
          disabled={!canManage}
          onCreated={refetch}
        />
      }
    >
      {!canManage ? (
        <OrganizationKeysNoPermission />
      ) : loading ? (
        // The empty list's frame, as the route's pending frame draws it, so
        // the section keeps its height while its keys are read (w1/m657 t007).
        <div>
          <span role="status" className="sr-only">
            {t("organizations.auditKeysLoading")}
          </span>
          <OrganizationApiKeysFrame />
        </div>
      ) : failed ? (
        <div role="alert" className="space-y-3">
          <p className="text-destructive text-sm">
            {t("organizations.auditKeysLoadFailed")}
          </p>
          <Button variant="outline" size="sm" onClick={refetch}>
            {t("organizations.auditKeysRetry")}
          </Button>
        </div>
      ) : (
        <div className="space-y-2 overflow-x-auto">
          <OrganizationApiKeysTable>
            {tokens.map((token) => (
              <OrganizationApiKeyRow
                key={token.id}
                token={token}
                onRevoke={() => setRevoking(token)}
              />
            ))}
          </OrganizationApiKeysTable>
          {tokens.length === 0 ? (
            <p className={ORGANIZATION_KEYS_EMPTY_CLASS}>
              {t("organizations.auditKeysEmpty")}
            </p>
          ) : null}
        </div>
      )}
      {revoking ? (
        <RevokeOrganizationApiKeyDialog
          organizationId={organizationId}
          token={revoking}
          onClose={() => setRevoking(null)}
          onRevoked={refetch}
        />
      ) : null}
    </OrganizationAuditLogKeysSection>
  );
}

/** One key: its name, its id as Render's key prefix, its ages, a menu. */
function OrganizationApiKeyRow({
  token,
  onRevoke,
}: {
  token: OrganizationApiToken;
  onRevoke: () => void;
}) {
  const { t } = useTranslations();
  const label = token.label || token.id;
  return (
    <TableRow>
      <TableCell className="max-w-0 truncate text-sm" title={label}>
        {token.label || "—"}
      </TableCell>
      <TableCell className="font-mono text-sm whitespace-nowrap">
        {token.id.slice(0, 8)}…
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge value={token.createdAt} />
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge
          value={token.lastUsedAt}
          fallback={t("organizations.auditKeysNeverUsed")}
        />
      </TableCell>
      <TableCell className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("organizations.auditKeysActions", { label })}
            >
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem variant="destructive" onSelect={onRevoke}>
              {t("organizations.auditKeysRevoke")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </TableCell>
    </TableRow>
  );
}

/** Render's revoke confirmation; a failure stays in the dialog. */
function RevokeOrganizationApiKeyDialog({
  organizationId,
  token,
  onClose,
  onRevoked,
}: {
  organizationId: string;
  token: OrganizationApiToken;
  onClose: () => void;
  onRevoked: () => void;
}) {
  const { t } = useTranslations();
  const { revoke, busy } = useRevokeOrganizationApiToken(organizationId);
  const [error, setError] = useState<string | null>(null);
  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
      closeOnConfirm={false}
      title={t("organizations.auditKeysRevokeTitle")}
      description={t("organizations.auditKeysRevokeBody", {
        label: token.label || token.id,
      })}
      cancelLabel={t("organizations.auditKeysRevokeCancel")}
      confirmLabel={t("organizations.auditKeysRevokeConfirm")}
      pending={busy}
      contentClassName="[overflow-wrap:anywhere]"
      onConfirm={() => {
        setError(null);
        revoke(token.id).then(
          () => {
            onRevoked();
            onClose();
          },
          () => setError(t("organizations.auditKeysRevokeFailed")),
        );
      }}
    >
      {error ? (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      ) : null}
    </ConfirmDialog>
  );
}

/**
 * Render's create flow: a name, Generate API key, then the key, shown once
 * with Done. The key lives only in this dialog's state (the mutation is not
 * cached) and goes when it closes.
 */
function CreateOrganizationApiKeyDialog({
  organizationId,
  disabled,
  onCreated,
}: {
  organizationId: string;
  disabled: boolean;
  onCreated: () => void;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [label, setLabel] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { provision, busy } = useProvisionOrganizationApiToken(organizationId);
  const submitting = useRef(false);
  const nameId = useId();

  function reset(next: boolean) {
    if (busy) return;
    setOpen(next);
    if (!next) {
      setLabel("");
      setToken(null);
      setError(null);
    }
  }

  async function generate() {
    const name = label.trim();
    if (!name || submitting.current || token) return;
    submitting.current = true;
    setError(null);
    try {
      setToken(await provision(name));
      onCreated();
    } catch (err) {
      setError(
        isKeyLimitRefusal(err)
          ? t("organizations.auditKeysTooMany")
          : t("organizations.auditKeysCreateFailed"),
      );
    } finally {
      submitting.current = false;
    }
  }

  return (
    <Dialog open={open} onOpenChange={reset}>
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        <Plus />
        {t("organizations.auditKeysCreate")}
      </Button>
      <DialogContent
        className="grid-cols-[minmax(0,1fr)]"
        onInteractOutside={(event) => {
          // The key is on screen only once: a stray click must not lose it.
          if (busy || token) event.preventDefault();
        }}
      >
        <form
          className="contents"
          onSubmit={(event) => {
            event.preventDefault();
            void generate();
          }}
        >
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {token ? (
                <CheckCircle2
                  aria-hidden="true"
                  className="size-5 shrink-0 text-green-600"
                />
              ) : null}
              {token
                ? t("organizations.auditKeysCreatedTitle")
                : t("organizations.auditKeysCreateTitle")}
            </DialogTitle>
            <DialogDescription>
              {token
                ? t("organizations.auditKeysSaveNow")
                : t("organizations.auditKeysCreateHint")}
            </DialogDescription>
          </DialogHeader>
          {token ? (
            <div role="group" aria-labelledby={nameId} className="space-y-1.5">
              <Label id={nameId}>{t("organizations.auditKeysKeyLabel")}</Label>
              <div className="bg-muted/50 flex items-center gap-2 rounded-md border p-3">
                <code className="min-w-0 flex-1 font-mono text-sm break-all">
                  {token}
                </code>
                <CopyButton
                  value={token}
                  label={t("organizations.auditKeysCopy")}
                  successText={t("organizations.auditKeysCopied")}
                  errorText={t("organizations.auditKeysCopyFailed")}
                />
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              <Label htmlFor={nameId} className="sr-only">
                {t("organizations.auditKeysNameLabel")}
              </Label>
              <div className="relative">
                <Input
                  id={nameId}
                  value={label}
                  maxLength={LABEL_MAX}
                  autoComplete="off"
                  autoFocus
                  required
                  disabled={busy}
                  placeholder={t("organizations.auditKeysNamePlaceholder")}
                  className="pr-14"
                  onChange={(event) => {
                    setLabel(event.target.value);
                    setError(null);
                  }}
                />
                <span
                  aria-hidden="true"
                  className="text-muted-foreground pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs tabular-nums"
                >
                  {label.length}/{LABEL_MAX}
                </span>
              </div>
            </div>
          )}
          {error ? (
            <p role="alert" className="text-destructive text-sm">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button
              type="button"
              variant={token ? "default" : "outline"}
              disabled={busy}
              onClick={() => reset(false)}
            >
              {token
                ? t("organizations.auditKeysDone")
                : t("organizations.auditKeysClose")}
            </Button>
            {token ? null : (
              <Button type="submit" disabled={!label.trim()} loading={busy}>
                {t("organizations.auditKeysGenerate")}
              </Button>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
