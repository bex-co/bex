import { useState } from "react";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useConnectionInfo } from "@/features/databases/hooks/use-connection-info";
import { CopyButton } from "@/common/components/copy-button";
import type { ConnectionInfoView } from "@/features/databases/types";

/**
 * Render's "Connections" panel. The connection strings + password are fetched
 * ONLY when the user clicks Reveal — never on mount (docs/bex-api.md §Managed
 * Postgres: the password is the one sensitive field, surfaced on demand). Until
 * then this shows just the button; after, each string gets a copy button and the
 * password is masked behind a show/hide toggle.
 */
export function ConnectionInfoPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const { info, loading, error, reveal, hide } = useConnectionInfo(id);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.connTitle")}</CardTitle>
        <CardDescription>{t("databases.connDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("databases.connErrorTitle")}</AlertTitle>
            <AlertDescription>{t("databases.connErrorBody")}</AlertDescription>
          </Alert>
        ) : null}

        {info ? (
          <>
            <RevealedInfo info={info} />
            <Button variant="outline" size="sm" onClick={hide}>
              {t("databases.connHide")}
            </Button>
          </>
        ) : (
          <Button onClick={() => void reveal()} disabled={loading}>
            {loading ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Eye />
            )}
            {t("databases.connReveal")}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

function RevealedInfo({ info }: { info: ConnectionInfoView }) {
  const { t } = useTranslations();
  return (
    <div className="space-y-4">
      <PasswordField value={info.password} />
      <ConnectionField
        label={t("databases.connInternal")}
        value={info.internalConnectionString}
      />
      {info.externalConnectionString ? (
        <ConnectionField
          label={t("databases.connExternal")}
          value={info.externalConnectionString}
        />
      ) : null}
      <ConnectionField
        label={t("databases.connPsql")}
        value={info.psqlCommand}
      />
    </div>
  );
}

/** A labeled, monospace, horizontally-scrollable connection string + copy. */
function ConnectionField({ label, value }: { label: string; value: string }) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium">{label}</span>
        <CopyButton
          value={value}
          label={label}
          successText={t("databases.copied")}
          errorText={t("databases.copyError")}
        />
      </div>
      <div className="overflow-x-auto rounded-md border bg-muted px-3 py-2">
        <code className="font-mono text-xs whitespace-pre">{value || "—"}</code>
      </div>
    </div>
  );
}

/** The password, masked by default with a show/hide toggle + copy. */
function PasswordField({ value }: { value: string }) {
  const { t } = useTranslations();
  const [shown, setShown] = useState(false);
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium">{t("databases.connPassword")}</span>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setShown((s) => !s)}
            aria-label={
              shown ? t("databases.connHidePassword") : t("databases.connShowPassword")
            }
          >
            {shown ? <EyeOff /> : <Eye />}
          </Button>
          <CopyButton
            value={value}
            label={t("databases.connPassword")}
            successText={t("databases.copied")}
            errorText={t("databases.copyError")}
          />
        </div>
      </div>
      <div className="overflow-x-auto rounded-md border bg-muted px-3 py-2">
        <code className="font-mono text-xs whitespace-pre">
          {shown ? value || "—" : "•".repeat(Math.max(value.length, 8))}
        </code>
      </div>
    </div>
  );
}
