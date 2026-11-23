import { useState } from "react";
import { Eye, Loader2 } from "lucide-react";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useConnectionInfo } from "@/features/keyvalue/hooks/use-connection-info";
import { useEnableKeyValueInternalAuth } from "@/features/keyvalue/hooks/use-enable-key-value-internal-auth";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { isAdminOnlyRefusal } from "@/features/capabilities/lib/admin-only";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { ConnectionField } from "@/common/components/connection-field";
import type { KeyValueConnectionInfoView } from "@/features/keyvalue/types";

/**
 * Render's "Connections" panel for a Key Value store. The connection strings
 * are fetched ONLY when the user clicks Reveal — never on mount
 * (docs/ADR021-keyvalue-management.md: the password lives inside the `redis://` URI
 * itself, so the whole string is the sensitive field; nothing is fetched or
 * rendered until the user asks). Until then this shows just the button; after,
 * each string gets its own copy button. Unlike databases' panel there is no
 * standalone password field to further mask — mirrors
 * `databases/connection-info-panel` minus its PasswordField.
 */
export function ConnectionInfoPanel({
  id,
  name,
  internalAuth,
  isPublic,
}: {
  id: string;
  name: string;
  internalAuth: boolean;
  /** Whether external access is enabled. The external URL is answered from
   *  the create, as on Render (w9/m521), for a private store too (w9/134);
   *  externalCertificate says whether its TLS listener accepts connections
   *  yet. */
  isPublic: boolean;
}) {
  const { t } = useTranslations();
  const { info, loading, error, reveal, hide } = useConnectionInfo(id);
  // Revealing the URIs is can_view_sensitive — the password lives inside the
  // `redis://` string, so the whole reveal is gated exactly like the databases
  // twin's connection strings + password (w9/m84), and in a protected
  // environment it is an Admin's alone (w4/m594).
  const capabilities = useCapabilities();
  const { canViewSensitive } = capabilities;
  const revealReasonKey = capabilities.reasonKey("can_view_sensitive");
  const adminOnly = useAdminOnlyActions("keyValue", id);
  const revealReason = revealReasonKey
    ? t(revealReasonKey)
    : adminOnly("view_connection_info");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.connTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.connDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("keyvalue.connErrorTitle")}</AlertTitle>
            <AlertDescription>
              {isAdminOnlyRefusal(error)
                ? t("capabilities.actionAdminOnly")
                : t("keyvalue.connErrorBody")}
            </AlertDescription>
          </Alert>
        ) : null}

        <InternalAuthSection id={id} name={name} enabled={internalAuth} />

        {info ? (
          <>
            <RevealedInfo info={info} isPublic={isPublic} />
            <Button variant="outline" size="sm" onClick={hide}>
              {t("keyvalue.connHide")}
            </Button>
          </>
        ) : (
          <PermissionTooltip reason={revealReason}>
            <Button
              onClick={() => void reveal()}
              disabled={loading || !canViewSensitive || revealReason != null}
            >
              {loading ? <Loader2 className="animate-spin" /> : <Eye />}
              {t("keyvalue.connReveal")}
            </Button>
          </PermissionTooltip>
        )}
      </CardContent>
    </Card>
  );
}

// Render's Info → Connections control: internal connections need no password
// until it is enabled, and enabling cannot be undone.
function InternalAuthSection({
  id,
  name,
  enabled,
}: {
  id: string;
  name: string;
  enabled: boolean;
}) {
  const { t } = useTranslations();
  const { enable, loading } = useEnableKeyValueInternalAuth();
  const [open, setOpen] = useState(false);
  const capabilities = useCapabilities();
  const reasonKey = capabilities.reasonKey("can_create");
  if (enabled) {
    return (
      <p className="text-sm text-muted-foreground">
        {t("keyvalue.internalAuthOn")}
      </p>
    );
  }
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <p className="text-sm text-muted-foreground">
        {t("keyvalue.internalAuthOff")}
      </p>
      <PermissionTooltip reason={reasonKey ? t(reasonKey) : undefined}>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setOpen(true)}
          disabled={loading || !capabilities.canCreate}
        >
          {t("keyvalue.internalAuthEnable")}
        </Button>
      </PermissionTooltip>
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={t("keyvalue.internalAuthConfirmTitle")}
        description={t("keyvalue.internalAuthConfirmBody")}
        cancelLabel={t("common.cancel")}
        confirmLabel={t("keyvalue.internalAuthEnable")}
        pending={loading}
        onConfirm={() => void enable(id, name)}
      />
    </div>
  );
}

function RevealedInfo({
  info,
  isPublic,
}: {
  info: KeyValueConnectionInfoView;
  isPublic: boolean;
}) {
  const { t } = useTranslations();
  const copiedText = t("keyvalue.copied");
  const copyErrorText = t("keyvalue.copyError");
  const internal = (
    <ConnectionField
      label={t("keyvalue.connInternal")}
      value={info.internalConnectionString}
      copiedText={copiedText}
      copyErrorText={copyErrorText}
    />
  );
  // A private store's API answer carries the external URL and a CLI command
  // over it, as Render's does (w9/134), but its endpoint admits nothing. Like
  // Render's Connections section, both fields say so instead of offering a
  // string that cannot connect.
  if (!isPublic) {
    const notAllowed = t("keyvalue.connExternalUnavailable");
    return (
      <div className="space-y-4">
        {internal}
        <UnavailableField
          label={t("keyvalue.connExternal")}
          note={notAllowed}
        />
        <UnavailableField label={t("keyvalue.connCli")} note={notAllowed} />
      </div>
    );
  }
  // A public store's endpoint that is not open yet: its URL is shown with a
  // note saying whether the certificate is still issuing or failed. An older
  // bex-api withheld the URL until then (w8/237): the note stands alone.
  const notOpen =
    info.externalCertificate === "pending" ||
    info.externalCertificate === "failed" ||
    !info.externalConnectionString;
  const notOpenNote = notOpen ? (
    <p className="text-sm text-muted-foreground">
      {t(
        info.externalCertificate === "failed"
          ? "keyvalue.connExternalFailed"
          : "keyvalue.connExternalPending",
      )}
    </p>
  ) : null;
  return (
    <div className="space-y-4">
      {internal}
      {info.externalConnectionString ? (
        <div className="space-y-2">
          <ConnectionField
            label={t("keyvalue.connExternal")}
            value={info.externalConnectionString}
            copiedText={copiedText}
            copyErrorText={copyErrorText}
          />
          {notOpenNote}
        </div>
      ) : (
        notOpenNote
      )}
      <ConnectionField
        label={t("keyvalue.connCli")}
        value={info.cliCommand}
        copiedText={copiedText}
        copyErrorText={copyErrorText}
      />
    </div>
  );
}

// A connection field with no value to copy, only a note saying why.
function UnavailableField({ label, note }: { label: string; note: string }) {
  return (
    <div className="space-y-1">
      <span className="text-sm font-medium">{label}</span>
      <p className="text-sm text-muted-foreground">{note}</p>
    </div>
  );
}
