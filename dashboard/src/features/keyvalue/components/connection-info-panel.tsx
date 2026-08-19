import { Eye, Loader2 } from "lucide-react";
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
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
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
export function ConnectionInfoPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const { info, loading, error, reveal, hide } = useConnectionInfo(id);
  // Revealing the URIs is can_view_sensitive — the password lives inside the
  // `redis://` string, so the whole reveal is gated exactly like the databases
  // twin's connection strings + password (w9/m84).
  const { canViewSensitive } = useCapabilities();
  const revealReason = canViewSensitive
    ? undefined
    : t("capabilities.reasonCanViewSensitive");

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
            <AlertDescription>{t("keyvalue.connErrorBody")}</AlertDescription>
          </Alert>
        ) : null}

        {info ? (
          <>
            <RevealedInfo info={info} />
            <Button variant="outline" size="sm" onClick={hide}>
              {t("keyvalue.connHide")}
            </Button>
          </>
        ) : (
          <PermissionTooltip reason={revealReason}>
            <Button
              onClick={() => void reveal()}
              disabled={loading || !canViewSensitive}
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

function RevealedInfo({ info }: { info: KeyValueConnectionInfoView }) {
  const { t } = useTranslations();
  const copiedText = t("keyvalue.copied");
  const copyErrorText = t("keyvalue.copyError");
  return (
    <div className="space-y-4">
      <ConnectionField
        label={t("keyvalue.connInternal")}
        value={info.internalConnectionString}
        copiedText={copiedText}
        copyErrorText={copyErrorText}
      />
      {info.externalConnectionString ? (
        <ConnectionField
          label={t("keyvalue.connExternal")}
          value={info.externalConnectionString}
          copiedText={copiedText}
          copyErrorText={copyErrorText}
        />
      ) : (
        <p className="text-sm text-muted-foreground">
          {t("keyvalue.connExternalUnavailable")}
        </p>
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
