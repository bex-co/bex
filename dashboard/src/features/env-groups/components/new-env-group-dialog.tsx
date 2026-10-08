import { useRef, useState } from "react";
import { FileUp, Loader2, Plus, Trash2, WandSparkles } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import { AutoTextarea } from "@/common/components/ui/auto-textarea";
import { isReservedEnvKey } from "@/features/services/lib/environment-draft";
import { Label } from "@/common/components/ui/label";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import { useEnvGroupMutations } from "@/features/env-groups/hooks/use-env-groups";
import {
  isValidEnvGroupName,
  isValidEnvVarKey,
  isValidSecretFileName,
} from "@/features/env-groups/lib/validation";
import type { ServiceView } from "@/features/services/types";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { ScopeSelect } from "@/features/env-groups/components/scope-select";
import {
  scopeEnvironmentId,
  scopeValue,
  serviceMatchesScope,
} from "@/features/env-groups/lib/scope";
import { EnvImportDialog } from "@/features/services/components/env-import-dialog";
import { ReservedEnvKeyNotice } from "@/features/services/components/reserved-env-key-notice";
import {
  upsertDotenvEntries,
  type DotenvEntry,
} from "@/features/services/lib/dotenv-import";

const EMPTY_SERVICE_ENVIRONMENTS: ReadonlyMap<string, string> = new Map();

interface EnvVarRow {
  id: number;
  key: string;
  value: string;
  generateValue: boolean;
}

interface SecretFileRow {
  id: number;
  name: string;
  content: string;
}

export interface NewEnvGroupDialogProps {
  onCreated: (id: string) => void;
  refetch?: () => Promise<unknown>;
  services?: ServiceView[];
  servicesLoading?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Service ids checked each time the dialog opens (service-page create path). */
  initialServiceIds?: string[];
  /**
   * Environment scope options and each service's environment — the same index
   * the detail page's link picker filters on. Callers with an unresolved index
   * must pass scopeReady=false; an empty resolved index means Workspace.
   */
  environments?: EnvironmentView[];
  /** The index's projects, naming each scope option's project (w4/215). */
  projects?: ProjectView[];
  serviceEnvironmentById?: ReadonlyMap<string, string>;
  /**
   * The scope the picker opens on: `null` is Workspace. The service-page path
   * passes the current service's environment so its pre-checked link is
   * compatible by construction (w4/m111 t002).
   */
  initialEnvironmentId?: string | null;
  /** The scope index is still loading — the picker waits rather than lying. */
  scopeLoading?: boolean;
  /** A complete index has resolved for the current workspace. */
  scopeReady?: boolean;
  scopeError?: Error;
  servicesError?: Error;
  onRetry?: () => void;
}

/** Each opening owns one draft, initialized only after its scope is known. */
export function NewEnvGroupDialog(props: NewEnvGroupDialogProps) {
  const { t } = useTranslations();
  const [openState, setOpenState] = useState(false);
  const controlled = props.open !== undefined;
  const open = props.open ?? openState;
  const onOpenChange = controlled ? props.onOpenChange : setOpenState;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {controlled ? null : (
        <DialogTrigger asChild>
          <Button size="sm">
            <Plus />
            {t("envGroups.newButton")}
          </Button>
        </DialogTrigger>
      )}
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("envGroups.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("envGroups.createDescription")}
          </DialogDescription>
        </DialogHeader>
        {open ? (
          <NewEnvGroupOpening
            {...props}
            onClose={() => onOpenChange?.(false)}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function NewEnvGroupOpening(
  props: NewEnvGroupDialogProps & { onClose: () => void },
) {
  const { t } = useTranslations();
  const {
    scopeLoading = false,
    scopeReady = !scopeLoading && !props.scopeError,
    servicesLoading = false,
    services = [],
    initialServiceIds = [],
  } = props;
  const missingService =
    !servicesLoading &&
    initialServiceIds.some(
      (id) => !services.some((service) => service.id === id),
    );
  const error = Boolean(
    props.scopeError || props.servicesError || missingService,
  );
  const waiting = !scopeReady || scopeLoading || servicesLoading;
  const canInitialize = !waiting && !error;
  const [initialized, setInitialized] = useState(canInitialize);
  // Latch readiness for this opening. A later poll or retry must keep the
  // mounted form (and the user's edits); closing unmounts the whole opening.
  if (!initialized && canInitialize) setInitialized(true);

  if (!initialized) {
    return (
      <>
        <CreateScopeStatus error={error} onRetry={props.onRetry} />
        <DialogFooter>
          <Button variant="outline" onClick={props.onClose}>
            {t("envGroups.cancel")}
          </Button>
          <Button disabled>{t("envGroups.createSubmit")}</Button>
        </DialogFooter>
      </>
    );
  }
  return (
    <NewEnvGroupForm
      {...props}
      unavailable={waiting || error}
      loadError={error}
    />
  );
}

function CreateScopeStatus({
  error,
  onRetry,
}: {
  error: boolean;
  onRetry?: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2 text-sm" role={error ? "alert" : "status"}>
      <p className={error ? "text-destructive" : "text-muted-foreground"}>
        {t(
          error ? "envGroups.createScopeError" : "envGroups.createScopeLoading",
        )}
      </p>
      {error && onRetry ? (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t("common.tryAgain")}
        </Button>
      ) : null}
    </div>
  );
}

/** One-step workspace create: name, initial contents, and links share one mutation. */
function NewEnvGroupForm({
  onCreated,
  onClose,
  refetch,
  services = [],
  initialServiceIds = [],
  environments = [],
  projects = [],
  serviceEnvironmentById = EMPTY_SERVICE_ENVIRONMENTS,
  initialEnvironmentId = null,
  unavailable,
  loadError,
  onRetry,
}: NewEnvGroupDialogProps & {
  onClose: () => void;
  unavailable: boolean;
  loadError: boolean;
}) {
  const { t } = useTranslations();
  const { createGroup, busy } = useEnvGroupMutations(refetch);
  const nextRowId = useRef(0);
  const [name, setName] = useState("");
  const [envVars, setEnvVars] = useState<EnvVarRow[]>([]);
  const [secretFiles, setSecretFiles] = useState<SecretFileRow[]>([]);
  const [serviceIds, setServiceIds] = useState<string[]>(initialServiceIds);
  const [scope, setScope] = useState(scopeValue(initialEnvironmentId));
  const [invalid, setInvalid] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const contentsValid =
    envVars.every((variable) => isValidEnvVarKey(variable.key)) &&
    secretFiles.every((file) => isValidSecretFileName(file.name));

  function addEnvVar() {
    setEnvVars((current) => [
      ...current,
      { id: nextRowId.current++, key: "", value: "", generateValue: false },
    ]);
  }

  function addSecretFile() {
    setSecretFiles((current) => [
      ...current,
      { id: nextRowId.current++, name: "", content: "" },
    ]);
  }

  function importEnvVars(entries: DotenvEntry[]) {
    setEnvVars((current) =>
      upsertDotenvEntries(
        current,
        entries,
        (row) => row.key,
        (row, entry) => ({
          ...row,
          value: entry.value,
          generateValue: false,
        }),
        (entry) => ({
          id: nextRowId.current++,
          key: entry.key,
          value: entry.value,
          generateValue: false,
        }),
      ),
    );
  }

  const environmentId = scopeEnvironmentId(scope);
  // bex-api refuses a link whose service lives in a different Environment than
  // the group (ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH). Offer only the links it
  // will accept rather than letting the user construct a create that cannot
  // succeed — which is what this dialog did for every in-Environment service
  // before w4/m111.
  const linkable = services.filter((service) =>
    serviceMatchesScope(serviceEnvironmentById, service.id, environmentId),
  );

  const linkableIds = new Set(linkable.map((service) => service.id));
  const selectedServiceIds = new Set(
    serviceIds.filter((id) => linkableIds.has(id)),
  );
  const scopeExists =
    environmentId === null ||
    environments.some((environment) => environment.id === environmentId);
  const canSubmit =
    isValidEnvGroupName(name) && !busy && !unavailable && scopeExists;

  function changeScope(next: string) {
    setScope(next);
    // Never submit a selection the new scope has made incompatible.
    const nextEnvironmentId = scopeEnvironmentId(next);
    setServiceIds((current) =>
      current.filter((id) =>
        serviceMatchesScope(serviceEnvironmentById, id, nextEnvironmentId),
      ),
    );
  }

  function toggleService(serviceId: string, checked: boolean) {
    setServiceIds((current) =>
      checked
        ? current.includes(serviceId)
          ? current
          : [...current, serviceId]
        : current.filter((id) => id !== serviceId),
    );
  }

  async function handleSubmit() {
    if (!canSubmit) return;
    if (!contentsValid) {
      setInvalid(true);
      return;
    }
    const id = await createGroup({
      name: name.trim(),
      envVars: envVars.map((variable) => ({
        key: variable.key.trim(),
        value: variable.generateValue ? undefined : variable.value,
        generateValue: variable.generateValue || undefined,
      })),
      secretFiles: secretFiles.map((file) => ({
        name: file.name.trim(),
        content: file.content,
      })),
      serviceIds: [...selectedServiceIds],
      environmentId,
    });
    if (!id) return;
    onClose();
    onCreated(id);
  }

  return (
    <>
      <div className="space-y-2">
        <Label htmlFor="env-group-name">{t("envGroups.nameLabel")}</Label>
        <Input
          id="env-group-name"
          value={name}
          onChange={(event) => {
            setName(event.target.value);
            setInvalid(false);
          }}
          placeholder={t("envGroups.namePlaceholder")}
          aria-invalid={invalid && !isValidEnvGroupName(name)}
          aria-describedby={
            invalid && !isValidEnvGroupName(name)
              ? "env-group-name-invalid"
              : undefined
          }
          autoComplete="off"
        />
        {invalid && !isValidEnvGroupName(name) ? (
          <p id="env-group-name-invalid" className="text-destructive text-sm">
            {t("envGroups.invalidName")}
          </p>
        ) : null}
      </div>

      <section className="space-y-3 rounded-md border p-4">
        <div>
          <h3 className="font-medium">{t("envGroups.createVarsTitle")}</h3>
          <p className="text-sm text-muted-foreground">
            {t("envGroups.createVarsDescription")}
          </p>
        </div>
        {envVars.map((variable, index) => (
          <div
            key={variable.id}
            className="grid gap-2 sm:grid-cols-[1fr_1fr_auto_auto]"
          >
            <div className="space-y-1">
              <Label htmlFor={`env-group-var-key-${variable.id}`}>
                {t("envGroups.varKeyLabel")}
              </Label>
              <Input
                id={`env-group-var-key-${variable.id}`}
                value={variable.key}
                onChange={(event) => {
                  const key = event.target.value;
                  setEnvVars((current) =>
                    current.map((row) =>
                      row.id === variable.id ? { ...row, key } : row,
                    ),
                  );
                  setInvalid(false);
                }}
                aria-invalid={
                  isReservedEnvKey(variable.key.trim()) ||
                  (invalid && !isValidEnvVarKey(variable.key))
                }
                placeholder={t("envGroups.varKeyPlaceholder")}
              />
              {/* bex owns PORT and refuses it server-side (w2/m95 t003) —
                    said the moment it is typed, not only on submit. */}
              {isReservedEnvKey(variable.key.trim()) ? (
                // A group can be linked to many services (or none yet), so
                // there is no single port to send the reader at — the notice
                // falls back to the services list (w4/m121/t003).
                <ReservedEnvKeyNotice
                  envKey={variable.key.trim()}
                  messageKey="envGroups.reservedKey"
                />
              ) : null}
            </div>
            <div className="space-y-1">
              <Label htmlFor={`env-group-var-value-${variable.id}`}>
                {t("envGroups.varValueLabel")}
              </Label>
              <AutoTextarea
                id={`env-group-var-value-${variable.id}`}
                masked
                value={variable.value}
                disabled={variable.generateValue}
                onChange={(event) => {
                  const value = event.target.value;
                  setEnvVars((current) =>
                    current.map((row) =>
                      row.id === variable.id ? { ...row, value } : row,
                    ),
                  );
                }}
                placeholder={t("envGroups.varValuePlaceholder")}
              />
            </div>
            <Button
              type="button"
              variant={variable.generateValue ? "secondary" : "outline"}
              className="self-end"
              aria-pressed={variable.generateValue}
              onClick={() =>
                setEnvVars((current) =>
                  current.map((row) =>
                    row.id === variable.id
                      ? { ...row, generateValue: !row.generateValue }
                      : row,
                  ),
                )
              }
            >
              <WandSparkles />
              {t("envGroups.generateValue")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="self-end"
              aria-label={t("envGroups.removeVar", { index: index + 1 })}
              onClick={() =>
                setEnvVars((current) =>
                  current.filter((row) => row.id !== variable.id),
                )
              }
            >
              <Trash2 />
            </Button>
          </div>
        ))}
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" size="sm" onClick={addEnvVar}>
            <Plus />
            {t("envGroups.addInitialVar")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setImportOpen(true)}
          >
            <FileUp /> {t("envGroups.importEnv")}
          </Button>
        </div>
      </section>

      <section className="space-y-3 rounded-md border p-4">
        <div>
          <h3 className="font-medium">{t("envGroups.createFilesTitle")}</h3>
          <p className="text-sm text-muted-foreground">
            {t("envGroups.createFilesDescription")}
          </p>
        </div>
        {secretFiles.map((file, index) => (
          <div key={file.id} className="grid gap-2 sm:grid-cols-[1fr_2fr_auto]">
            <div className="space-y-1">
              <Label htmlFor={`env-group-file-name-${file.id}`}>
                {t("envGroups.fileNameLabel")}
              </Label>
              <Input
                id={`env-group-file-name-${file.id}`}
                value={file.name}
                onChange={(event) => {
                  const fileName = event.target.value;
                  setSecretFiles((current) =>
                    current.map((row) =>
                      row.id === file.id ? { ...row, name: fileName } : row,
                    ),
                  );
                  setInvalid(false);
                }}
                aria-invalid={invalid && !isValidSecretFileName(file.name)}
                placeholder={t("envGroups.fileNamePlaceholder")}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor={`env-group-file-content-${file.id}`}>
                {t("envGroups.fileContentLabel")}
              </Label>
              <Textarea
                id={`env-group-file-content-${file.id}`}
                value={file.content}
                onChange={(event) => {
                  const content = event.target.value;
                  setSecretFiles((current) =>
                    current.map((row) =>
                      row.id === file.id ? { ...row, content } : row,
                    ),
                  );
                }}
                placeholder={t("envGroups.fileContentPlaceholder")}
              />
            </div>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="self-end"
              aria-label={t("envGroups.removeFile", { index: index + 1 })}
              onClick={() =>
                setSecretFiles((current) =>
                  current.filter((row) => row.id !== file.id),
                )
              }
            >
              <Trash2 />
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={addSecretFile}
        >
          <Plus />
          {t("envGroups.addInitialFile")}
        </Button>
      </section>

      <section className="space-y-3 rounded-md border p-4">
        <div>
          <h3 className="font-medium">{t("envGroups.createServicesTitle")}</h3>
          <p className="text-sm text-muted-foreground">
            {t("envGroups.createServicesDescription")}
          </p>
        </div>
        {unavailable || !scopeExists ? (
          <CreateScopeStatus
            error={loadError || !scopeExists}
            onRetry={onRetry}
          />
        ) : null}
        <ScopeSelect
          id="env-group-create-scope"
          value={scope}
          environments={environments}
          projects={projects}
          loading={busy || unavailable}
          onValueChange={changeScope}
        />
        {linkable.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {services.length === 0
              ? t("envGroups.noServicesToLink")
              : t("envGroups.noServicesInScope")}
          </p>
        ) : (
          <div className="grid gap-2 sm:grid-cols-2">
            {linkable.map((service) => {
              const checkboxId = `env-group-service-${service.id}`;
              return (
                <div
                  key={service.id}
                  className="flex items-center gap-2 rounded-md border p-3"
                >
                  <Checkbox
                    id={checkboxId}
                    checked={selectedServiceIds.has(service.id)}
                    disabled={busy || unavailable}
                    onCheckedChange={(checked) =>
                      toggleService(service.id, checked === true)
                    }
                  />
                  <Label
                    htmlFor={checkboxId}
                    className="min-w-0 cursor-pointer"
                  >
                    <span className="block truncate">{service.name}</span>
                    {service.name !== service.id ? (
                      <span className="block truncate font-mono text-xs text-muted-foreground">
                        {service.id}
                      </span>
                    ) : null}
                  </Label>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {invalid && contentsValid === false ? (
        <p className="text-destructive text-sm" role="alert">
          {t("envGroups.invalidInitialContents")}
        </p>
      ) : null}

      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={busy}>
          {t("envGroups.cancel")}
        </Button>
        <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("envGroups.createSubmit")}
        </Button>
      </DialogFooter>
      <EnvImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImport={importEnvVars}
      />
    </>
  );
}
