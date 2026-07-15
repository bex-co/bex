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
        <AllowListSection key={entryKey(access.allowList)} access={access} />
        <UsersSection access={access} />
        <PoolerSection access={access} />
      </CardContent>
    </Card>
  );
}

type Access = ReturnType<typeof useAccessControl>;

// One canonical serialization of an entry list — used both as the section's
// remount key and for the dirty check, so the two can't disagree.
function entryKey(list: { cidrBlock: string; description: string }[]) {
  return list.map((e) => `${e.cidrBlock}=${e.description}`).join(",");
}

function AllowListSection({ access }: { access: Access }) {
  const { t } = useTranslations();
  // Local editable copy, seeded from the server list; dirty until saved.
  // Entries carry {cidrBlock, description} — the description persists end to
  // end (w4/m24), so the label a human gives an entry survives the round-trip.
  const [draft, setDraft] = useState(access.allowList);
  const [entry, setEntry] = useState("");
  const [description, setDescription] = useState("");

  const dirty = entryKey(draft) !== entryKey(access.allowList);

  function add() {
    const c = entry.trim();
    if (c && !draft.some((e) => e.cidrBlock === c)) {
      setDraft([...draft, { cidrBlock: c, description: description.trim() }]);
    }
    setEntry("");
    setDescription("");
  }

  return (
    <section className="space-y-2">
      <h4 className="text-sm font-medium">{t("databases.accessAllowList")}</h4>
      <p className="text-xs text-muted-foreground">
        {t("databases.accessAllowListHint")}
      </p>
      <div className="flex flex-wrap gap-2">
        {draft.length === 0 ? (
          <span className="text-sm text-muted-foreground">
            {t("databases.accessAllowListOpen")}
          </span>
        ) : (
          draft.map((e) => (
            <span
              key={e.cidrBlock}
              className="inline-flex items-center gap-1 rounded-md border bg-muted px-2 py-1 text-xs"
            >
              <code className="font-mono">{e.cidrBlock}</code>
              {e.description ? (
                <span className="text-muted-foreground">{e.description}</span>
              ) : null}
              <button
                type="button"
                aria-label={t("databases.accessAllowListRemove", {
                  cidr: e.cidrBlock,
                })}
                onClick={() =>
                  setDraft(draft.filter((x) => x.cidrBlock !== e.cidrBlock))
                }
                className="text-muted-foreground hover:text-foreground"
              >
                <Trash2 className="size-3" />
              </button>
            </span>
          ))
        )}
      </div>
      <div className="flex flex-wrap gap-2">
        <Input
          value={entry}
          onChange={(e) => setEntry(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), add())}
          placeholder="203.0.113.0/24"
          className="max-w-xs"
        />
        <Input
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), add())}
          placeholder={t("databases.accessAllowListDescription")}
          className="max-w-xs"
        />
        <Button
          variant="outline"
          size="sm"
          onClick={add}
          disabled={!entry.trim()}
        >
          <Plus />
          {t("databases.accessAllowListAdd")}
        </Button>
        <Button
          size="sm"
          onClick={() => void access.saveAllowList(draft)}
          disabled={!dirty || access.savingAllowList}
        >
          {access.savingAllowList ? <Loader2 className="animate-spin" /> : null}
          {t("databases.accessAllowListSave")}
        </Button>
      </div>
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
