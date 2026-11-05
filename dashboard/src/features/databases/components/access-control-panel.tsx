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
import { ConnectionField } from "@/common/components/connection-field";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { ipAllowListEntryKey } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAccessControl } from "@/features/databases/hooks/use-access-control";
import { isReservedPostgresRole } from "@/features/databases/lib/identifiers";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";

/**
 * The database detail's Access Control section: the external-endpoint IP
 * allowlist (editable CIDR list), Render's credentials (a new one becomes the
 * default; the default can't be deleted), and an on-demand reveal of the
 * pooled connection strings. Mirrors the services env-vars panel shape.
 */
export function AccessControlPanel({
  id,
  plan,
}: {
  id: string;
  plan: string | null;
}) {
  const { t } = useTranslations();
  const access = useAccessControl(id);
  const { instanceTypes } = useDatabaseInstanceTypes();
  const supportsPooling = instanceTypes.find(
    (it) => it.id === plan,
  )?.supportsConnectionPooling;

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
        <PoolerSection access={access} supportsPooling={supportsPooling} />
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
        allowUnchangedSave={
          access.isPublic != null &&
          access.isPublic !== Boolean(access.allowList.length)
        }
        labels={{
          hint: t("databases.accessAllowListHint"),
          empty: t(
            access.isPublic && access.allowList.length === 0
              ? "databases.accessAllowListPublicEmpty"
              : "databases.accessAllowListEmpty",
          ),
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

  // A reserved role wedged the database's reconcile (w4/m170); bex-api refuses
  // it too, but say so before the click.
  const reserved = isReservedPostgresRole(name.trim());

  async function create() {
    if (await access.createCredential(name.trim())) {
      setName("");
    }
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
              onClick={() => void access.deleteCredential(c.username)}
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
    </section>
  );
}

function PoolerSection({
  access,
  supportsPooling,
}: {
  access: Access;
  supportsPooling: boolean | undefined;
}) {
  const { t } = useTranslations();
  return (
    <section className="space-y-2">
      <h4 className="text-sm font-medium">{t("databases.accessPooler")}</h4>
      <p className="text-xs text-muted-foreground">
        {t("databases.accessPoolerHint")}
      </p>
      {access.pooled ? (
        access.pooled.internal || access.pooled.external ? (
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
        ) : (
          <p className="text-sm text-muted-foreground">
            {t(
              supportsPooling === false
                ? "databases.accessPoolerPlanUnsupported"
                : "databases.accessPoolerDisabled",
            )}
          </p>
        )
      ) : (
        <Button
          variant="outline"
          size="sm"
          onClick={() => void access.revealPooled()}
          disabled={access.poolLoading}
        >
          {access.poolLoading ? <Loader2 className="animate-spin" /> : <Eye />}
          {t("databases.accessPoolerReveal")}
        </Button>
      )}
    </section>
  );
}
