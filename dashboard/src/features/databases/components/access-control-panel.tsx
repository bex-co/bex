import { useState } from "react";
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
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { CopyButton } from "@/common/components/copy-button";
import { ConnectionField } from "@/common/components/connection-field";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";
import { ipAllowListEntryKey } from "@/common/lib/ip-allow-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAccessControl } from "@/features/databases/hooks/use-access-control";

/**
 * The database detail's Access Control section: the external-endpoint IP
 * allowlist (editable CIDR list), the additional Postgres login roles (create
 * reveals the password once, delete), and an on-demand reveal of the pooled
 * connection strings. Mirrors the services env-vars panel shape.
 */
export function AccessControlPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const access = useAccessControl(id);

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
        <UsersSection access={access} />
        <PoolerSection access={access} />
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
      <IPAllowListEditor
        entries={access.allowList}
        saving={access.savingAllowList}
        onSave={access.saveAllowList}
        labels={{
          hint: t("databases.accessAllowListHint"),
          open: t("databases.accessAllowListOpen"),
          descriptionPlaceholder: t("databases.accessAllowListDescription"),
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

function UsersSection({ access }: { access: Access }) {
  const { t } = useTranslations();
  const [name, setName] = useState("");
  const [revealed, setRevealed] = useState<{
    name: string;
    password: string;
  } | null>(null);

  async function create() {
    const pw = await access.createUser(name.trim());
    if (pw != null) {
      setRevealed({ name: name.trim(), password: pw });
      setName("");
    }
  }

  return (
    <section className="space-y-2">
      <h4 className="text-sm font-medium">{t("databases.accessUsers")}</h4>
      <p className="text-xs text-muted-foreground">
        {t("databases.accessUsersHint")}
      </p>
      {access.users.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {t("databases.accessUsersEmpty")}
        </p>
      ) : (
        <ul className="space-y-1">
          {access.users.map((u) => (
            <li
              key={u}
              className="flex items-center justify-between gap-2 rounded-md border px-3 py-2"
            >
              <code className="font-mono text-sm">{u}</code>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("databases.accessUserDelete", { name: u })}
                onClick={() => void access.deleteUser(u)}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      )}
      {revealed ? (
        <Alert>
          <AlertDescription className="space-y-1">
            <span className="text-sm font-medium">
              {t("databases.accessUserPasswordOnce", { name: revealed.name })}
            </span>
            <div className="flex items-center justify-between gap-2 rounded-md border bg-muted px-2 py-1">
              <code className="truncate font-mono text-xs">
                {revealed.password}
              </code>
              <CopyButton
                value={revealed.password}
                label={t("databases.accessUserPassword")}
                successText={t("databases.copied")}
                errorText={t("databases.copyError")}
              />
            </div>
          </AlertDescription>
        </Alert>
      ) : null}
      <div className="flex gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="reporting"
          className="max-w-xs"
        />
        <Button
          variant="outline"
          size="sm"
          onClick={() => void create()}
          disabled={!name.trim() || access.creatingUser}
        >
          {access.creatingUser ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Plus />
          )}
          {t("databases.accessUserAdd")}
        </Button>
      </div>
    </section>
  );
}

function PoolerSection({ access }: { access: Access }) {
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
            {t("databases.accessPoolerDisabled")}
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
