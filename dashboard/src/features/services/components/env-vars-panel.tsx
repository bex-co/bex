import { useState } from "react";
import { Plus, KeyRound, ShieldAlert, AlertTriangle } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
} from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useEnvVarKeys,
  useRevealEnvVar,
  useEnvVarMutations,
  classifyEnvVarError,
} from "@/features/services/hooks/use-env-vars";
import { EnvVarRow } from "@/features/services/components/env-var-row";
import { CenteredState } from "@/features/services/components/centered-state";

// A C-locale env-var name: what bex-api (and a shell, and Kubernetes' Secret-key
// validation) accepts — reject bad names client-side rather than round-tripping
// to a 400. Kept in sync with backend/internal/secrets validEnvKey.
const VALID_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;

/**
 * The service Environment tab (Render dashboard shape): lists a service's env-var
 * keys, reveals a value on demand, and adds/updates/deletes variables — all over
 * bex-api's env-vars GraphQL (docs/ADR006-bex-api.md#env-vars). Values are fetched per
 * key, never in the list.
 */
export function EnvVarsPanel({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { keys, loading, error, refetch } = useEnvVarKeys(serviceId);
  const reveal = useRevealEnvVar(serviceId);
  const { setVar, deleteVar, busy } = useEnvVarMutations(serviceId, refetch);

  const errorKind = classifyEnvVarError(error);
  const initialLoading = loading && keys.length === 0 && !error;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.envTitle")}</CardTitle>
        <CardDescription>{t("services.envDescription")}</CardDescription>
        <CardAction>
          <AddVarButton setVar={setVar} disabled={gated || busy} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <TableSkeleton />
        ) : keys.length === 0 ? (
          <EnvEmptyState />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-1/3">
                  {t("services.envColKey")}
                </TableHead>
                <TableHead>{t("services.envColValue")}</TableHead>
                <TableHead className="sr-only text-right">actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.map((entry) => (
                <EnvVarRow
                  key={entry.id}
                  entry={entry}
                  reveal={reveal}
                  onSave={setVar}
                  onDelete={deleteVar}
                  busy={busy}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

/** The "Add variable" affordance: a button that opens an inline key+value form. */
function AddVarButton({
  setVar,
  disabled,
}: {
  setVar: (key: string, value: string) => Promise<boolean>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [invalid, setInvalid] = useState(false);
  const [saving, setSaving] = useState(false);

  function reset() {
    setKey("");
    setValue("");
    setInvalid(false);
    setOpen(false);
  }

  async function submit() {
    if (!VALID_KEY.test(key.trim())) {
      setInvalid(true);
      return;
    }
    setSaving(true);
    const ok = await setVar(key.trim(), value);
    setSaving(false);
    if (ok) reset();
  }

  if (!open) {
    return (
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        <Plus /> {t("services.envAdd")}
      </Button>
    );
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-2">
        <Input
          value={key}
          onChange={(e) => {
            setKey(e.target.value);
            setInvalid(false);
          }}
          placeholder={t("services.envKeyPlaceholder")}
          aria-label={t("services.envColKey")}
          aria-invalid={invalid}
          className="w-40 font-mono text-sm"
          autoFocus
        />
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={t("services.envValuePlaceholder")}
          aria-label={t("services.envColValue")}
          className="w-40 font-mono text-sm"
          onKeyDown={(e) => {
            if (e.key === "Enter") void submit();
            if (e.key === "Escape") reset();
          }}
        />
        <Button size="sm" disabled={saving} onClick={() => void submit()}>
          {t("services.envSave")}
        </Button>
        <Button size="sm" variant="ghost" onClick={reset}>
          {t("services.envCancel")}
        </Button>
      </div>
      {invalid && (
        <p className="text-destructive text-xs">
          {t("services.envInvalidKey")}
        </p>
      )}
    </div>
  );
}

function TableSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex items-center gap-4">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-6 flex-1" />
        </div>
      ))}
    </div>
  );
}

function EnvEmptyState() {
  const { t } = useTranslations();
  return (
    <CenteredState
      icon={<KeyRound />}
      title={t("services.envEmptyTitle")}
      body={t("services.envEmptyBody")}
    />
  );
}

/** The unavailable (503) / forbidden (403) / generic error states. */
function StatePanel({
  kind,
}: {
  kind: "unavailable" | "forbidden" | "generic";
}) {
  const { t } = useTranslations();
  const copy = {
    unavailable: {
      icon: <AlertTriangle />,
      title: t("services.envUnavailableTitle"),
      body: t("services.envUnavailableBody"),
    },
    forbidden: {
      icon: <ShieldAlert />,
      title: t("services.envForbiddenTitle"),
      body: t("services.envForbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("services.envErrorTitle"),
      body: t("services.envErrorBody"),
    },
  }[kind];
  return <CenteredState icon={copy.icon} title={copy.title} body={copy.body} />;
}
