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
import type { EnvVarKey } from "@/features/services/types";

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
  return (
    <EnvVarsEditor
      keys={keys}
      loading={loading}
      errorKind={errorKind}
      reveal={reveal}
      setVar={setVar}
      deleteVar={deleteVar}
      busy={busy}
      copy={{
        title: t("services.envTitle"),
        description: t("services.envDescription"),
        emptyTitle: t("services.envEmptyTitle"),
        emptyBody: t("services.envEmptyBody"),
        unavailableTitle: t("services.envUnavailableTitle"),
        unavailableBody: t("services.envUnavailableBody"),
        forbiddenTitle: t("services.envForbiddenTitle"),
        forbiddenBody: t("services.envForbiddenBody"),
        errorTitle: t("services.envErrorTitle"),
        errorBody: t("services.envErrorBody"),
        deleteConfirmBody: t("services.envDeleteConfirmBody"),
      }}
    />
  );
}

export type SensitiveEditorErrorKind = "unavailable" | "forbidden" | "generic";

export interface EnvVarsEditorCopy {
  title: string;
  description: string;
  emptyTitle: string;
  emptyBody: string;
  unavailableTitle: string;
  unavailableBody: string;
  forbiddenTitle: string;
  forbiddenBody: string;
  errorTitle: string;
  errorBody: string;
  deleteConfirmBody: string;
}

/** Shared keys-only env-var editor used by services and workspace env groups. */
export function EnvVarsEditor({
  keys,
  loading,
  errorKind,
  reveal,
  setVar,
  deleteVar,
  busy,
  copy,
}: {
  keys: EnvVarKey[];
  loading: boolean;
  errorKind: SensitiveEditorErrorKind | null;
  reveal: (key: string) => Promise<string>;
  setVar: (key: string, value: string) => Promise<boolean>;
  deleteVar: (key: string) => Promise<boolean>;
  busy: boolean;
  copy: EnvVarsEditorCopy;
}) {
  const { t } = useTranslations();
  const initialLoading = loading && keys.length === 0 && !errorKind;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{copy.title}</CardTitle>
        <CardDescription>{copy.description}</CardDescription>
        <CardAction>
          <AddVarButton setVar={setVar} disabled={gated || busy} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} copy={copy} />
        ) : initialLoading ? (
          <TableSkeleton />
        ) : keys.length === 0 ? (
          <CenteredState
            icon={<KeyRound />}
            title={copy.emptyTitle}
            body={copy.emptyBody}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-1/3">
                  {t("services.envColKey")}
                </TableHead>
                <TableHead>{t("services.envColValue")}</TableHead>
                <TableHead className="sr-only text-right">
                  {t("services.actions")}
                </TableHead>
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
                  deleteConfirmBody={copy.deleteConfirmBody}
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

/** The unavailable (503) / forbidden (403) / generic error states. */
function StatePanel({
  kind,
  copy,
}: {
  kind: SensitiveEditorErrorKind;
  copy: EnvVarsEditorCopy;
}) {
  const state = {
    unavailable: {
      icon: <AlertTriangle />,
      title: copy.unavailableTitle,
      body: copy.unavailableBody,
    },
    forbidden: {
      icon: <ShieldAlert />,
      title: copy.forbiddenTitle,
      body: copy.forbiddenBody,
    },
    generic: {
      icon: <AlertTriangle />,
      title: copy.errorTitle,
      body: copy.errorBody,
    },
  }[kind];
  return (
    <CenteredState icon={state.icon} title={state.title} body={state.body} />
  );
}
