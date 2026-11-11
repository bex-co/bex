import { useState, useId } from "react";
import { Loader2, Plus, Trash2, Eye } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Badge } from "@/common/components/ui/badge";
import { Switch } from "@/common/components/ui/switch";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { ConnectionField } from "@/common/components/connection-field";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { ipAllowListEntryKey } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAccessControl } from "@/features/databases/hooks/use-access-control";
import { isReservedPostgresRole } from "@/features/databases/lib/identifiers";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useUpdateDatabaseConnectionPool } from "@/features/databases/hooks/use-update-database-connection-pool";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { DatabaseDetailView } from "@/features/databases/types";

export interface AccessControlPanelProps {
  database: Pick<
    DatabaseDetailView,
    "id" | "name" | "plan" | "poolerEnabled" | "highAvailabilityEnabled"
  >;
  /** Called after a connection-pool toggle is accepted, to re-read the page. */
  onChanged: () => void;
}

/**
 * The database detail's Access Control section: the external-endpoint IP
 * allowlist (editable CIDR list), Render's credentials (a new one becomes the
 * default; the default can't be deleted), Render's Connection Pool toggle
 * (w2/m307) and an on-demand reveal of the pooled connection strings. Mirrors
 * the services env-vars panel shape.
 */
export function AccessControlPanel({
  database,
  onChanged,
}: AccessControlPanelProps) {
  const { t } = useTranslations();
  const access = useAccessControl(database.id);
  const { instanceTypes } = useDatabaseInstanceTypes();
  const planUnsupported =
    instanceTypes.find((it) => it.id === database.plan)
      ?.supportsConnectionPooling === false;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.accessTitle")}</CardTitle>
        <CardDescription>{t("databases.accessDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* key remounts the editable draft from the server list whenever it
            changes (e.g. after a save), avoiding an effect-based state sync. */}
        <AllowListSection
          key={ipAllowListEntryKey(access.allowList)}
          access={access}
        />
        <CredentialsSection access={access} />
        <PoolerSection
          database={database}
          access={access}
          planUnsupported={planUnsupported}
          onChanged={onChanged}
        />
      </CardContent>
    </Card>
  );
}

type Access = ReturnType<typeof useAccessControl>;

function AllowListSection({ access }: { access: Access }) {
  const { t } = useTranslations();
  return (
    <section className="space-y-2">
      <h4 className="text-sm font-medium">{t("databases.accessAllowList")}</h4>
      {access.isPublic === false ? (
        <p className="text-xs text-muted-foreground">
          {t("databases.accessAllowListInternalOnly")}
        </p>
      ) : null}
      <IPAllowListEditor
        entries={access.allowList}
        saving={access.savingAllowList}
        onSave={access.saveAllowList}
        labels={{
          hint: t("databases.accessAllowListHint"),
          empty: t("databases.accessAllowListEmpty"),
          descriptionPlaceholder: t("databases.accessAllowListDescription"),
          cidr: t("databases.accessAllowListCIDR"),
          cidrRule: (number) =>
            t("databases.accessAllowListCIDRRule", { number }),
          description: t("databases.accessAllowListDescriptionLabel"),
          descriptionRule: (number) =>
            t("databases.accessAllowListDescriptionRule", { number }),
          add: t("databases.accessAllowListAdd"),
          save: t("databases.accessAllowListSave"),
          invalid: t("databases.accessAllowListInvalid"),
          remove: (cidr) => t("databases.accessAllowListRemove", { cidr }),
          moveUp: (cidr) => t("databases.accessAllowListMoveUp", { cidr }),
          moveDown: (cidr) => t("databases.accessAllowListMoveDown", { cidr }),
        }}
      />
    </section>
  );
}

// Passwords are read through the connection info, never shown here (Render).
function CredentialsSection({ access }: { access: Access }) {
  const { t } = useTranslations();
  const reservedId = useId();
  const [name, setName] = useState("");
  // Retiring a login breaks every client on it at once, so Delete asks first,
  // as the page's other destructive controls do (w4/551). The name outlives
  // the open flag so the dialog keeps it while it fades out.
  const [doomed, setDoomed] = useState<string | null>(null);
  const [confirming, setConfirming] = useState(false);

  // A reserved role wedged the database's reconcile (w4/m170); bex-api refuses
  // it too, but say so before the click.
  const reserved = isReservedPostgresRole(name.trim());

  async function create() {
    if (await access.createCredential(name.trim())) {
      setName("");
    }
  }

  async function deleteDoomed(username: string) {
    if (await access.deleteCredential(username)) setConfirming(false);
  }

  return (
    <section className="space-y-2">
      <h4 className="text-sm font-medium">
        {t("databases.accessCredentials")}
      </h4>
      <p className="text-xs text-muted-foreground">
        {t("databases.accessCredentialsHint")}
      </p>
      <ul className="space-y-1">
        {access.credentials.map((c) => (
          <li
            key={c.username}
            className="flex items-center justify-between gap-2 rounded-md border px-3 py-2"
          >
            <span className="flex min-w-0 items-center gap-2">
              <code className="truncate font-mono text-sm">{c.username}</code>
              {c.default ? (
                <Badge variant="secondary">
                  {t("databases.accessCredentialDefault")}
                </Badge>
              ) : null}
            </span>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t("databases.accessCredentialDelete", {
                name: c.username,
              })}
              title={
                c.default
                  ? t("databases.accessCredentialDefaultLocked")
                  : undefined
              }
              disabled={c.default}
              onClick={() => {
                setDoomed(c.username);
                setConfirming(true);
              }}
            >
              <Trash2 />
            </Button>
          </li>
        ))}
      </ul>
      <div className="flex gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="app_user_2"
          aria-label={t("databases.accessCredentialName")}
          aria-invalid={reserved}
          aria-describedby={reserved ? reservedId : undefined}
          className="max-w-xs"
        />
        <Button
          variant="outline"
          size="sm"
          onClick={() => void create()}
          disabled={!name.trim() || reserved || access.creatingCredential}
        >
          {access.creatingCredential ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Plus />
          )}
          {t("databases.accessCredentialAdd")}
        </Button>
      </div>
      {reserved ? (
        <p id={reservedId} role="alert" className="text-sm text-destructive">
          {t("databases.fieldPhysicalNameReserved", { name: name.trim() })}
        </p>
      ) : null}
      <ConfirmDialog
        open={confirming}
        onOpenChange={(open) => {
          if (!open && !access.deletingCredential) setConfirming(false);
        }}
        title={t("databases.accessCredentialDeleteTitle", {
          name: doomed ?? "",
        })}
        description={t("databases.accessCredentialDeleteBody", {
          name: doomed ?? "",
        })}
        confirmLabel={t("databases.accessCredentialDeleteConfirm")}
        pending={access.deletingCredential}
        closeOnConfirm={false}
        onConfirm={() => {
          if (doomed != null) void deleteDoomed(doomed);
        }}
      />
    </section>
  );
}

// Render's Info > General > Connection Pool (w2/m307): a switch that asks
// first, because either direction restarts the database (or fails over to the
// standby when HA is on). Free plans can't pool, so enabling is refused there;
// a pooler left on can always be turned off, as bex-api allows.
function PoolerSection({
  database,
  access,
  planUnsupported,
  onChanged,
}: AccessControlPanelProps & { access: Access; planUnsupported: boolean }) {
  const { id, name, poolerEnabled, highAvailabilityEnabled } = database;
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const operateReasonKey = capabilities.reasonKey("can_operate");
  const operateReason = operateReasonKey ? t(operateReasonKey) : undefined;
  const { updateConnectionPool, busy } = useUpdateDatabaseConnectionPool();
  const [target, setTarget] = useState<boolean | null>(null);
  const hasPooledStrings = Boolean(
    access.pooled?.internal || access.pooled?.external,
  );

  async function handleToggle(enabled: boolean) {
    if (await updateConnectionPool(id, name, enabled)) {
      setTarget(null);
      access.resetPooled();
      onChanged();
    }
  }

  return (
    <section className="space-y-2">
      <div className="flex items-center justify-between gap-4">
        <h4 className="text-sm font-medium">{t("databases.accessPooler")}</h4>
        <PermissionTooltip reason={operateReason}>
          <Switch
            aria-label={t("databases.poolerToggleLabel")}
            checked={poolerEnabled}
            disabled={
              operateReason != null ||
              busy ||
              (!poolerEnabled && planUnsupported)
            }
            onCheckedChange={setTarget}
          />
        </PermissionTooltip>
      </div>
      <p className="text-xs text-muted-foreground">
        {t("databases.accessPoolerHint")}
      </p>
      {access.pooled && hasPooledStrings ? (
        <div className="space-y-2">
          <ConnectionField
            label={t("databases.accessPoolerInternal")}
            value={access.pooled.internal}
            copiedText={t("databases.copied")}
            copyErrorText={t("databases.copyError")}
          />
          {access.pooled.external ? (
            <ConnectionField
              label={t("databases.accessPoolerExternal")}
              value={access.pooled.external}
              copiedText={t("databases.copied")}
              copyErrorText={t("databases.copyError")}
            />
          ) : null}
        </div>
      ) : !poolerEnabled ? (
        <p className="text-sm text-muted-foreground">
          {t(
            planUnsupported
              ? "databases.accessPoolerPlanUnsupported"
              : "databases.accessPoolerDisabled",
          )}
        </p>
      ) : (
        <>
          {access.pooled ? (
            // Enabled, but the operator hasn't published the pooler's hosts
            // yet; Reveal stays so the strings can be read once it has.
            <p className="text-sm text-muted-foreground">
              {t("databases.accessPoolerStarting")}
            </p>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            onClick={() => void access.revealPooled()}
            disabled={access.poolLoading}
          >
            {access.poolLoading ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Eye />
            )}
            {t("databases.accessPoolerReveal")}
          </Button>
        </>
      )}

      <ConfirmDialog
        open={target != null}
        onOpenChange={(open) => {
          if (!open && !busy) setTarget(null);
        }}
        title={t(
          target
            ? "databases.poolerEnableTitle"
            : "databases.poolerDisableTitle",
          { name },
        )}
        description={t(
          target
            ? highAvailabilityEnabled
              ? "databases.poolerEnableBodyHA"
              : "databases.poolerEnableBody"
            : highAvailabilityEnabled
              ? "databases.poolerDisableBodyHA"
              : "databases.poolerDisableBody",
        )}
        confirmLabel={t(
          target
            ? "databases.poolerEnableConfirm"
            : "databases.poolerDisableConfirm",
        )}
        destructive={!target}
        pending={busy}
        closeOnConfirm={false}
        onConfirm={() => {
          if (target != null) void handleToggle(target);
        }}
      />
    </section>
  );
}
