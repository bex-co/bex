import { useState } from "react";
import { toast } from "sonner";
import {
  Download,
  Plus,
  KeyRound,
  ShieldAlert,
  AlertTriangle,
  Sparkles,
} from "lucide-react";
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
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useEnvVarKeys,
  useRevealEnvVar,
  useEnvVarMutations,
  classifyEnvVarError,
} from "@/features/services/hooks/use-env-vars";
import { EnvVarRow } from "@/features/services/components/env-var-row";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import type { EnvVarKey } from "@/features/services/types";
import {
  downloadEnvFile,
  formatEnvExport,
} from "@/features/services/lib/env-export";

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
      serviceId={serviceId}
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
  serviceId,
  keys,
  loading,
  errorKind,
  reveal,
  setVar,
  deleteVar,
  busy,
  copy,
}: {
  serviceId?: string;
  keys: EnvVarKey[];
  loading: boolean;
  errorKind: SensitiveEditorErrorKind | null;
  reveal: (key: string) => Promise<string>;
  setVar: (
    key: string,
    value: string,
    generateValue?: boolean,
  ) => Promise<boolean>;
  deleteVar: (key: string) => Promise<boolean>;
  busy: boolean;
  copy: EnvVarsEditorCopy;
}) {
  const { t } = useTranslations();
  // Reveal is can_view_sensitive; add/edit/delete are can_create.
  const { canCreate, canViewSensitive } = useCapabilities();
  const revealReason = canViewSensitive
    ? undefined
    : t("capabilities.reasonCanViewSensitive");
  const createReason = !canCreate
    ? t("capabilities.reasonCanCreate")
    : undefined;
  const initialLoading = loading && keys.length === 0 && !errorKind;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{copy.title}</CardTitle>
        <CardDescription>{copy.description}</CardDescription>
        <CardAction>
          <div className="flex items-center gap-2">
            {serviceId ? (
              <ExportEnvButton
                serviceId={serviceId}
                keys={keys}
                reveal={reveal}
                disabled={loading || errorKind != null}
              />
            ) : null}
            <AddVarButton
              setVar={setVar}
              disabled={gated || busy || !canCreate}
              disabledReason={createReason}
            />
          </div>
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} copy={copy} />
        ) : initialLoading ? (
          <PanelTableSkeleton rows={3} />
        ) : keys.length === 0 ? (
          <PanelCenteredState
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
                  canReveal={canViewSensitive}
                  revealReason={revealReason}
                  canCreate={canCreate}
                  createReason={createReason}
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

/**
 * Fetches every value network-only through `reveal` before creating a file.
 * Promise.all is deliberately fail-closed: one masked/unavailable value means
 * no download, so the file can never look complete while silently omitting a
 * secret or substituting a UI mask.
 */
export function ExportEnvButton({
  serviceId,
  keys,
  reveal,
  disabled,
}: {
  serviceId: string;
  keys: EnvVarKey[];
  reveal: (key: string) => Promise<string>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [exporting, setExporting] = useState(false);

  async function exportEnvironment() {
    setExporting(true);
    try {
      const values = await Promise.all(
        keys.map(async ({ key }) => ({ key, value: await reveal(key) })),
      );
      downloadEnvFile(`${serviceId}.env`, formatEnvExport(values));
      toast.success(t("services.envExportSuccess"));
    } catch {
      toast.error(t("services.envExportError"));
    } finally {
      setExporting(false);
    }
  }

  return (
    <Button
      variant="outline"
      size="sm"
      disabled={disabled || exporting}
      onClick={() => void exportEnvironment()}
    >
      <Download /> {t("services.envExport")}
    </Button>
  );
}

/** The "Add variable" affordance: a button that opens an inline key+value form. */
function AddVarButton({
  setVar,
  disabled,
  disabledReason,
}: {
  setVar: (
    key: string,
    value: string,
    generateValue?: boolean,
  ) => Promise<boolean>;
  disabled: boolean;
  disabledReason?: string;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [generateValue, setGenerateValue] = useState(false);
  const [invalid, setInvalid] = useState(false);
  const [saving, setSaving] = useState(false);

  function reset() {
    setKey("");
    setValue("");
    setGenerateValue(false);
    setInvalid(false);
    setOpen(false);
  }

  async function submit() {
    if (!VALID_KEY.test(key.trim())) {
      setInvalid(true);
      return;
    }
    setSaving(true);
    const ok = generateValue
      ? await setVar(key.trim(), "", true)
      : await setVar(key.trim(), value);
    setSaving(false);
    if (ok) reset();
  }

  if (!open) {
    return (
      <PermissionTooltip reason={disabled ? disabledReason : undefined}>
        <Button
          variant="outline"
          size="sm"
          disabled={disabled}
          onClick={() => setOpen(true)}
        >
          <Plus /> {t("services.envAdd")}
        </Button>
      </PermissionTooltip>
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
          placeholder={
            generateValue
              ? t("services.envGeneratePlaceholder")
              : t("services.envValuePlaceholder")
          }
          aria-label={t("services.envColValue")}
          className="w-40 font-mono text-sm"
          disabled={generateValue}
          onKeyDown={(e) => {
            if (e.key === "Enter") void submit();
            if (e.key === "Escape") reset();
          }}
        />
        <Button
          size="sm"
          variant={generateValue ? "secondary" : "outline"}
          aria-pressed={generateValue}
          onClick={() => {
            setGenerateValue((current) => !current);
            setValue("");
          }}
        >
          <Sparkles /> {t("services.envGenerate")}
        </Button>
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
    <PanelCenteredState
      icon={state.icon}
      title={state.title}
      body={state.body}
    />
  );
}
