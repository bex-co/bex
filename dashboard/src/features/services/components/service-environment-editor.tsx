import { useId, useMemo, useRef, useState } from "react";
import { useBlocker } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  Check,
  ChevronDown,
  Clipboard,
  Download,
  Eye,
  EyeOff,
  FilePlus2,
  FileUp,
  Loader2,
  Pencil,
  Plus,
  RotateCw,
  Sparkles,
  Trash2,
  Undo2,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Badge } from "@/common/components/ui/badge";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  classifyEnvVarError,
  useEnvVarKeys,
  useRevealEnvVar,
  type EnvVarErrorKind,
} from "@/features/services/hooks/use-env-vars";
import {
  classifySecretFileError,
  useRevealSecretFile,
  useSecretFileNames,
} from "@/features/services/hooks/use-secret-files";
import { useEnvironmentDraftSave } from "@/features/services/hooks/use-environment-draft-save";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import {
  MAX_SECRET_FILE_BYTES,
  createEnvironmentDraft,
  environmentDraftPatch,
  isDraftValid,
  isEnvironmentDraftDirty,
  isNewDraftRow,
  isValidSecretFileName,
  MASKED_VALUE,
  validateEnvironmentDraft,
  type EnvDraftRow,
  type EnvironmentDraft,
  type SecretFileDraftRow,
  type EnvironmentPatchInput,
} from "@/features/services/lib/environment-draft";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";
import {
  downloadEnvFile,
  formatEnvExport,
} from "@/features/services/lib/env-export";
import {
  upsertDotenvEntries,
  type DotenvEntry,
} from "@/features/services/lib/dotenv-import";
import { useSensitiveReveals } from "@/features/services/hooks/use-sensitive-reveals";
import { EnvImportDialog } from "./env-import-dialog";
import { SecretFileContentDialog } from "./secret-file-content-dialog";

type SaveChoice = "only" | "deploy" | "rebuild";

// Spelled out rather than assembled from the kind, so every key is greppable
// and a new EnvVarErrorKind fails to compile instead of rendering its own key.
const ENV_ERROR_COPY: Record<EnvVarErrorKind, { title: string; body: string }> =
  {
    generic: { title: "services.envErrorTitle", body: "services.envErrorBody" },
    forbidden: {
      title: "services.envForbiddenTitle",
      body: "services.envForbiddenBody",
    },
    unavailable: {
      title: "services.envUnavailableTitle",
      body: "services.envUnavailableBody",
    },
  };

export function ServiceEnvironmentEditor({ serviceId }: { serviceId: string }) {
  const env = useEnvVarKeys(serviceId);
  const files = useSecretFileNames(serviceId);
  const revealEnv = useRevealEnvVar(serviceId);
  const revealFile = useRevealSecretFile(serviceId);
  const { save, saving } = useEnvironmentDraftSave();
  const { trigger, deploying } = useTriggerDeploy();

  return (
    <EnvironmentEditor
      resourceId={serviceId}
      envKeys={env.keys}
      secretFileNames={files.names}
      loading={
        (env.loading && env.keys.length === 0) ||
        (files.loading && files.names.length === 0)
      }
      errorKind={
        classifyEnvVarError(env.error) ?? classifySecretFileError(files.error)
      }
      revealEnv={revealEnv}
      revealFile={revealFile}
      saving={saving || deploying}
      save={async (patch, choice) => {
        await save(
          serviceId,
          patch,
          choice === "deploy" ? "deploy" : "save_only",
        );
        if (choice !== "rebuild") {
          return {
            affectedServiceIds: choice === "deploy" ? [serviceId] : [],
          };
        }
        return {
          affectedServiceIds: [serviceId],
          rolloutFailed: (await trigger(serviceId)) == null,
        };
      }}
      retryRollout={async () => (await trigger(serviceId)) != null}
    />
  );
}

export interface EnvironmentEditorProps {
  resourceId: string;
  envKeys: Array<{ id: string; key: string }>;
  secretFileNames: Array<{ id: string; name: string }>;
  loading: boolean;
  errorKind: EnvVarErrorKind | null;
  revealEnv: (key: string) => Promise<string>;
  revealFile: (name: string) => Promise<string>;
  save: (
    patch: EnvironmentPatchInput,
    choice: SaveChoice,
  ) => Promise<{
    affectedServiceIds?: readonly string[];
    failedServiceIds?: readonly string[];
    rolloutFailed?: boolean;
  }>;
  retryRollout: (choice: Exclude<SaveChoice, "only">) => Promise<boolean>;
  saving: boolean;
  generateOnServer?: boolean;
}

/**
 * The staged, masked environment editor shared by services and environment
 * groups. Resource-specific hooks stay in thin wrappers; draft validation,
 * sparse patches, import/export, navigation blocking, and rollout recovery are
 * intentionally one UI contract.
 */
export function EnvironmentEditor({
  resourceId,
  envKeys,
  secretFileNames,
  loading,
  errorKind,
  revealEnv,
  revealFile,
  save,
  retryRollout,
  saving,
  generateOnServer = false,
}: EnvironmentEditorProps) {
  const { t } = useTranslations();
  const {
    canCreate,
    canViewSensitive,
    loaded: capabilitiesLoaded,
  } = useCapabilities();
  const createDenied = capabilitiesLoaded && !canCreate;
  const sensitiveDenied = capabilitiesLoaded && !canViewSensitive;
  const createReason = createDenied
    ? t("capabilities.reasonCanCreate")
    : undefined;
  const sensitiveReason = sensitiveDenied
    ? t("capabilities.reasonCanViewSensitive")
    : undefined;
  const writeReasonID = useId();
  const nextID = useRef(0);
  const fileInput = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState<EnvironmentDraft | null>(null);
  const reveals = useSensitiveReveals(
    { env: revealEnv, file: revealFile },
    (kind) =>
      toast.error(
        t(
          kind === "env"
            ? "services.envRevealError"
            : "services.secretFileRevealError",
        ),
      ),
  );
  const [importOpen, setImportOpen] = useState(false);
  const [contentRowID, setContentRowID] = useState<string | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState(false);
  const [pendingChoice, setPendingChoice] = useState<Exclude<
    SaveChoice,
    "only"
  > | null>(null);
  const [exporting, setExporting] = useState(false);

  const validation = useMemo(
    () => (draft ? validateEnvironmentDraft(draft) : { env: {}, files: {} }),
    [draft],
  );
  const patch = useMemo(
    () =>
      draft ? environmentDraftPatch(draft) : { envVars: [], secretFiles: [] },
    [draft],
  );
  const dirty = draft ? isEnvironmentDraftDirty(draft) : false;
  const busy = saving;
  const blocker = useBlocker({
    shouldBlockFn: () => dirty,
    enableBeforeUnload: dirty,
    withResolver: true,
  });

  function beginEdit() {
    if (createDenied) return;
    setDraft(
      createEnvironmentDraft(
        envKeys.map((entry) => entry.key),
        secretFileNames.map((entry) => entry.name),
      ),
    );
    setSaveError(false);
    setUploadError(null);
  }

  // Deleting a row that was never on the server drops it outright; deleting a
  // server-backed one only stages it, so the save can send the delete.
  function updateRow<K extends "envVars" | "secretFiles">(
    list: K,
    id: string,
    update: Partial<EnvironmentDraft[K][number]>,
  ) {
    if (createDenied) return;
    setDraft((current) =>
      current
        ? {
            ...current,
            [list]: (
              current[list] as Array<EnvironmentDraft[K][number]>
            ).flatMap((row) =>
              row.id !== id
                ? [row]
                : isNewDraftRow(row) && update.deleted
                  ? []
                  : [{ ...row, ...update }],
            ),
          }
        : current,
    );
  }

  function addVariable(generated = false) {
    if (!draft || createDenied) return;
    const existing = new Set(draft.envVars.map((row) => row.key));
    let key = generated ? "NEW_SECRET" : "";
    let suffix = 2;
    while (key && existing.has(key)) key = `NEW_SECRET_${suffix++}`;
    const id = `new-env:${nextID.current++}`;
    setDraft((current) =>
      current
        ? {
            ...current,
            envVars: [
              ...current.envVars,
              {
                id,
                originalKey: null,
                key,
                value: generated && !generateOnServer ? generateEnvValue() : "",
                valueChanged: true,
                generateValue: generated && generateOnServer,
                deleted: false,
              },
            ],
          }
        : current,
    );
  }

  function importVariables(entries: DotenvEntry[]) {
    if (createDenied) return;
    setDraft((current) => {
      if (!current) return current;
      return {
        ...current,
        envVars: upsertDotenvEntries(
          current.envVars,
          entries,
          (row) => (row.deleted ? null : row.key),
          (row, entry) => ({
            ...row,
            value: entry.value,
            valueChanged: true,
            generateValue: false,
          }),
          (entry) => ({
            id: `import-env:${nextID.current++}`,
            originalKey: null,
            key: entry.key,
            value: entry.value,
            valueChanged: true,
            deleted: false,
          }),
        ),
      };
    });
  }

  function addSecretFile() {
    if (createDenied) return;
    setDraft((current) =>
      current
        ? {
            ...current,
            secretFiles: [
              ...current.secretFiles,
              {
                id: `new-file:${nextID.current++}`,
                originalName: null,
                name: "",
                content: "",
                contentChanged: true,
                deleted: false,
              },
            ],
          }
        : current,
    );
  }

  async function uploadFiles(selected: FileList | null) {
    if (!selected || !draft || createDenied) return;
    const names = new Set(
      draft.secretFiles
        .filter((row) => !row.deleted)
        .map((row) => row.name.trim()),
    );
    const additions: SecretFileDraftRow[] = [];
    let rejected = false;
    for (const file of Array.from(selected)) {
      const name = file.name.split(/[\\/]/).at(-1) ?? "";
      if (
        !isValidSecretFileName(name) ||
        names.has(name) ||
        file.size > MAX_SECRET_FILE_BYTES
      ) {
        rejected = true;
        continue;
      }
      try {
        const content = await file.text();
        if (content.includes("\0")) {
          rejected = true;
          continue;
        }
        names.add(name);
        additions.push({
          id: `upload-file:${nextID.current++}`,
          originalName: null,
          name,
          content,
          contentChanged: true,
          deleted: false,
        });
      } catch {
        rejected = true;
      }
    }
    if (additions.length) {
      setDraft((current) =>
        current
          ? { ...current, secretFiles: [...current.secretFiles, ...additions] }
          : current,
      );
    }
    setUploadError(rejected ? t("services.secretFileUploadError") : null);
  }

  async function exportEnvironment(kind: "copy" | "download") {
    if (sensitiveDenied) return;
    setExporting(true);
    try {
      const values = await Promise.all(
        envKeys.map(async ({ key }) => ({ key, value: await revealEnv(key) })),
      );
      const formatted = formatEnvExport(values);
      if (kind === "copy") {
        await navigator.clipboard.writeText(formatted);
        toast.success(t("services.envCopySuccess"));
      } else {
        downloadEnvFile(`${resourceId}.env`, formatted);
        toast.success(t("services.envExportSuccess"));
      }
    } catch {
      toast.error(t("services.envExportError"));
    } finally {
      setExporting(false);
    }
  }

  async function commit(choice: SaveChoice) {
    if (createDenied || !draft || !dirty || !isDraftValid(validation)) return;
    setSaveError(false);
    let result: {
      affectedServiceIds?: readonly string[];
      failedServiceIds?: readonly string[];
      rolloutFailed?: boolean;
    };
    try {
      result = await save(patch, choice);
    } catch {
      setSaveError(true);
      toast.error(t("services.environmentSaveError"));
      return;
    }

    // The configuration save is committed at this point. End the draft before
    // any refresh/deploy follow-up so a later failure can never cause a retry to
    // reapply an already-successful secret patch.
    setDraft(null);
    reveals.clear();
    const rolloutFailed =
      result.rolloutFailed || Boolean(result.failedServiceIds?.length);
    if (choice !== "only" && rolloutFailed) {
      setPendingChoice(choice);
      return;
    }
    if (choice !== "rebuild") {
      const deployed =
        choice === "deploy" && Boolean(result.affectedServiceIds?.length);
      toast.success(
        deployed
          ? t("services.environmentSaveDeploySuccess")
          : t("services.environmentSaveOnlySuccess"),
      );
    }
    setPendingChoice(null);
  }

  async function retryDeploy() {
    if (!pendingChoice || createDenied) return;
    if (await retryRollout(pendingChoice)) {
      setPendingChoice(null);
    }
  }

  const contentRow = draft?.secretFiles.find((row) => row.id === contentRowID);

  async function copyFresh(kind: "env" | "file", name: string) {
    if (sensitiveDenied) return;
    try {
      const value = await (kind === "env" ? revealEnv(name) : revealFile(name));
      await navigator.clipboard.writeText(value);
      toast.success(t("services.envCopySuccess"));
    } catch {
      toast.error(
        t(
          kind === "env"
            ? "services.envRevealError"
            : "services.secretFileRevealError",
        ),
      );
    }
  }

  return (
    <div className="space-y-6">
      {pendingChoice ? (
        <Alert>
          <RotateCw />
          <AlertTitle>
            {t("services.environmentSavedDeployFailedTitle")}
          </AlertTitle>
          <AlertDescription>
            <p>{t("services.environmentSavedDeployFailedBody")}</p>
            <Button
              size="sm"
              variant="outline"
              disabled={saving || createDenied}
              onClick={() => void retryDeploy()}
            >
              {saving ? <Loader2 className="animate-spin" /> : <RotateCw />}
              {t("services.environmentRetryDeploy")}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {errorKind ? (
        <Alert variant="destructive">
          <AlertTitle>{t(ENV_ERROR_COPY[errorKind].title)}</AlertTitle>
          <AlertDescription>
            {t(ENV_ERROR_COPY[errorKind].body)}
          </AlertDescription>
        </Alert>
      ) : null}

      {draft && createDenied ? (
        <p
          id={writeReasonID}
          className="text-muted-foreground text-sm"
          role="status"
        >
          {createReason}
        </p>
      ) : null}

      <EnvironmentSection
        title={t("services.envTitle")}
        description={t("services.envDescription")}
        empty={{
          title: t("services.envEmptyTitle"),
          body: t("services.envEmptyBody"),
        }}
        loading={loading}
        action={
          <>
            <PermissionTooltip reason={sensitiveReason}>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={
                      loading ||
                      Boolean(errorKind) ||
                      exporting ||
                      sensitiveDenied
                    }
                  >
                    {exporting ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <Download />
                    )}
                    {t("services.envExport")}
                    <ChevronDown />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    onSelect={() => void exportEnvironment("copy")}
                  >
                    <Clipboard /> {t("services.envCopy")}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onSelect={() => void exportEnvironment("download")}
                  >
                    <Download /> {t("services.envDownload")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </PermissionTooltip>
            {!draft ? (
              <PermissionTooltip reason={createReason}>
                <Button
                  size="sm"
                  disabled={loading || Boolean(errorKind) || createDenied}
                  onClick={beginEdit}
                >
                  <Pencil /> {t("services.environmentEdit")}
                </Button>
              </PermissionTooltip>
            ) : (
              <PermissionTooltip reason={createReason}>
                <AddVariableMenu
                  disabled={createDenied}
                  onAdd={() => addVariable(false)}
                  onGenerate={() => addVariable(true)}
                  onImport={() => {
                    if (!createDenied) setImportOpen(true);
                  }}
                />
              </PermissionTooltip>
            )}
          </>
        }
        isEmpty={(draft ? draft.envVars : envKeys).length === 0}
      >
        {draft
          ? draft.envVars.map((row) => (
              <EnvDraftItem
                key={row.id}
                row={row}
                error={validation.env[row.id]}
                disabled={createDenied}
                permissionDescriptionID={writeReasonID}
                onChange={(update) => updateRow("envVars", row.id, update)}
              />
            ))
          : envKeys.map(({ id, key }) => (
              <SensitiveViewItem
                key={id}
                name={key}
                value={sensitiveDenied ? undefined : reveals.value("env", key)}
                loading={reveals.busy("env", key)}
                revealDisabled={sensitiveDenied}
                revealReason={sensitiveReason}
                onToggle={() => {
                  if (!sensitiveDenied) reveals.toggle("env", key);
                }}
                onCopy={() => copyFresh("env", key)}
              />
            ))}
      </EnvironmentSection>

      <EnvironmentSection
        title={t("services.secretFilesTitle")}
        description={t("services.secretFilesDescription")}
        empty={{
          title: t("services.secretFilesEmptyTitle"),
          body: t("services.secretFilesEmptyBody"),
        }}
        loading={loading}
        notice={uploadError}
        action={
          draft ? (
            <>
              <PermissionTooltip reason={createReason}>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={createDenied}
                  onClick={addSecretFile}
                >
                  <FilePlus2 /> {t("services.secretFileAdd")}
                </Button>
              </PermissionTooltip>
              <PermissionTooltip reason={createReason}>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={createDenied}
                  onClick={() => fileInput.current?.click()}
                >
                  <FileUp /> {t("services.secretFileUpload")}
                </Button>
              </PermissionTooltip>
              <input
                ref={fileInput}
                className="sr-only"
                type="file"
                multiple
                disabled={createDenied}
                onChange={(event) => {
                  void uploadFiles(event.target.files);
                  event.target.value = "";
                }}
              />
            </>
          ) : null
        }
        isEmpty={(draft ? draft.secretFiles : secretFileNames).length === 0}
      >
        {draft
          ? draft.secretFiles.map((row) => (
              <FileDraftItem
                key={row.id}
                row={row}
                error={validation.files[row.id]}
                disabled={createDenied}
                permissionDescriptionID={writeReasonID}
                onChange={(update) => updateRow("secretFiles", row.id, update)}
                onContent={() => {
                  if (!createDenied) setContentRowID(row.id);
                }}
              />
            ))
          : secretFileNames.map(({ id, name }) => (
              <SensitiveViewItem
                key={id}
                name={name}
                value={
                  sensitiveDenied ? undefined : reveals.value("file", name)
                }
                loading={reveals.busy("file", name)}
                revealDisabled={sensitiveDenied}
                revealReason={sensitiveReason}
                onToggle={() => {
                  if (!sensitiveDenied) reveals.toggle("file", name);
                }}
                onCopy={() => copyFresh("file", name)}
              />
            ))}
      </EnvironmentSection>

      {draft ? (
        <Card className="sticky bottom-3 z-20 gap-4 py-4 shadow-lg">
          <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="font-medium">
                {t("services.environmentUnsavedTitle")}
              </p>
              <p className="text-muted-foreground text-sm" aria-live="polite">
                {t("services.environmentUnsavedSummary", {
                  variables: patch.envVars.length,
                  files: patch.secretFiles.length,
                })}
              </p>
              {saveError ? (
                <p className="text-destructive text-sm" role="alert">
                  {t("services.environmentSaveError")}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => setDraft(null)}
              >
                {t("services.envCancel")}
              </Button>
              <PermissionTooltip reason={createReason}>
                <div className="flex">
                  <Button
                    className="flex-1 rounded-r-none sm:flex-none"
                    disabled={
                      !dirty ||
                      !isDraftValid(validation) ||
                      busy ||
                      createDenied
                    }
                    onClick={() => void commit("deploy")}
                  >
                    {busy ? <Loader2 className="animate-spin" /> : <Check />}
                    {t("services.environmentSaveDeploy")}
                  </Button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        className="rounded-l-none border-l border-primary-foreground/30 px-2"
                        disabled={
                          !dirty ||
                          !isDraftValid(validation) ||
                          busy ||
                          createDenied
                        }
                        aria-label={t("services.environmentSaveOptions")}
                      >
                        <ChevronDown />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onSelect={() => void commit("rebuild")}>
                        {t("services.environmentSaveRebuild")}
                      </DropdownMenuItem>
                      <DropdownMenuItem onSelect={() => void commit("deploy")}>
                        {t("services.environmentSaveDeploy")}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onSelect={() => void commit("only")}>
                        {t("services.environmentSaveOnly")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </PermissionTooltip>
            </div>
          </CardContent>
        </Card>
      ) : null}

      <EnvImportDialog
        open={importOpen && !createDenied}
        onOpenChange={(open) => {
          if (!createDenied) setImportOpen(open);
        }}
        onImport={importVariables}
      />
      {contentRow ? (
        <SecretFileContentDialog
          open
          name={contentRow.name || t("services.secretFileUntitled")}
          content={contentRow.content}
          disabled={createDenied}
          reveal={
            contentRow.originalName && !sensitiveDenied && !createDenied
              ? () => revealFile(contentRow.originalName as string)
              : undefined
          }
          onOpenChange={(open) => {
            if (!open) setContentRowID(null);
          }}
          onSave={(content, changed) =>
            updateRow("secretFiles", contentRow.id, {
              content,
              contentChanged: contentRow.contentChanged || changed,
            })
          }
        />
      ) : null}

      <AlertDialog open={blocker.status === "blocked"}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("services.environmentDiscardTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("services.environmentDiscardBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => blocker.reset?.()}>
              {t("services.environmentKeepEditing")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setDraft(null);
                blocker.proceed?.();
              }}
            >
              {t("services.environmentDiscard")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

/**
 * One titled card of environment rows. The env-var and secret-file halves of
 * this page are the same card down to the class strings — header, optional
 * action cluster, loading skeleton, divided row list, empty copy — and differ
 * only in their copy and which rows they render.
 */
function EnvironmentSection({
  title,
  description,
  action,
  notice,
  loading,
  isEmpty,
  empty,
  children,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
  notice?: string | null;
  loading: boolean;
  isEmpty: boolean;
  empty: { title: string; body: string };
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="grid-cols-1 grid-rows-none sm:grid-cols-[minmax(0,1fr)_auto] sm:grid-rows-[auto_auto]">
        <div>
          <CardTitle>{title}</CardTitle>
          <CardDescription className="mt-1.5">{description}</CardDescription>
        </div>
        {action ? (
          <CardAction className="col-start-1 row-start-2 mt-2 justify-self-stretch sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
            <div className="flex flex-wrap gap-2 sm:justify-end">{action}</div>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent>
        {notice ? (
          <p className="text-destructive mb-3 text-sm" role="alert">
            {notice}
          </p>
        ) : null}
        {loading ? (
          <RowsSkeleton />
        ) : (
          <div className="divide-y">
            {children}
            {isEmpty ? (
              <EmptyCopy title={empty.title} body={empty.body} />
            ) : null}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function SensitiveViewItem({
  name,
  value,
  loading,
  revealDisabled,
  revealReason,
  onToggle,
  onCopy,
}: {
  name: string;
  value: string | undefined;
  loading: boolean;
  revealDisabled: boolean;
  revealReason?: string;
  onToggle: () => void;
  onCopy: () => Promise<void>;
}) {
  const { t } = useTranslations();
  const visible = value !== undefined;
  return (
    <div className="grid min-w-0 gap-3 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-center">
      <code className="min-w-0 break-all text-sm font-medium">{name}</code>
      <code
        className="min-w-0 break-all text-sm"
        aria-label={visible ? value : t("services.environmentMaskedValue")}
      >
        {visible ? value : MASKED_VALUE}
      </code>
      <div className="flex flex-wrap justify-end gap-1">
        <PermissionTooltip
          reason={!visible && revealDisabled ? revealReason : undefined}
        >
          <Button
            size="sm"
            variant="ghost"
            disabled={loading || (!visible && revealDisabled)}
            onClick={onToggle}
          >
            {loading ? (
              <Loader2 className="animate-spin" />
            ) : visible ? (
              <EyeOff />
            ) : (
              <Eye />
            )}
            {visible ? t("services.envHide") : t("services.envReveal")}
          </Button>
        </PermissionTooltip>
        <PermissionTooltip reason={revealDisabled ? revealReason : undefined}>
          <Button
            size="icon"
            variant="ghost"
            disabled={loading || revealDisabled}
            aria-label={t("services.envCopy")}
            onClick={() => void onCopy()}
          >
            <Clipboard />
          </Button>
        </PermissionTooltip>
      </div>
    </div>
  );
}

/** A row staged for deletion on save, with the Undo affordance that reverts it. */
function StagedDeleteRow({
  label,
  disabled,
  permissionDescriptionID,
  onUndo,
}: {
  // Nullable: a row staged for delete before its server name is known renders
  // an empty label rather than crashing.
  label: string | null;
  disabled: boolean;
  permissionDescriptionID?: string;
  onUndo: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex min-w-0 items-center justify-between gap-3 py-4 first:pt-0 last:pb-0">
      <code className="min-w-0 break-all text-sm line-through opacity-60">
        {label}
      </code>
      <div className="flex items-center gap-2">
        <Badge variant="outline">{t("services.environmentStagedDelete")}</Badge>
        <Button
          size="sm"
          variant="ghost"
          disabled={disabled}
          aria-describedby={disabled ? permissionDescriptionID : undefined}
          onClick={onUndo}
        >
          <Undo2 /> {t("services.environmentUndo")}
        </Button>
      </div>
    </div>
  );
}

function EnvDraftItem({
  row,
  error,
  disabled,
  permissionDescriptionID,
  onChange,
}: {
  row: EnvDraftRow;
  error?: "invalid" | "duplicate" | "value";
  disabled: boolean;
  permissionDescriptionID?: string;
  onChange: (update: Partial<EnvDraftRow>) => void;
}) {
  const { t } = useTranslations();
  if (row.deleted) {
    return (
      <StagedDeleteRow
        label={row.originalKey}
        disabled={disabled}
        permissionDescriptionID={permissionDescriptionID}
        onUndo={() => onChange({ deleted: false })}
      />
    );
  }
  return (
    <div className="grid min-w-0 gap-2 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start">
      <div className="min-w-0 space-y-1">
        <Input
          value={row.key}
          disabled={disabled}
          aria-describedby={disabled ? permissionDescriptionID : undefined}
          onChange={(event) => onChange({ key: event.target.value })}
          className="min-w-0 font-mono text-sm"
          aria-label={t("services.envColKey")}
          aria-invalid={Boolean(error)}
          placeholder={t("services.envKeyPlaceholder")}
        />
        {error ? (
          <p className="text-destructive text-xs" role="alert">
            {error === "duplicate"
              ? t("services.environmentDuplicateKey")
              : error === "value"
                ? t("services.environmentValueRequired")
                : t("services.envInvalidKey")}
          </p>
        ) : null}
      </div>
      <Input
        value={row.value ?? ""}
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        onChange={(event) =>
          onChange({
            value: event.target.value,
            valueChanged: true,
            generateValue: false,
          })
        }
        className="min-w-0 font-mono text-sm"
        aria-label={t("services.envColValue")}
        placeholder={
          row.originalKey && !row.valueChanged
            ? t("services.environmentUnchangedMasked")
            : t("services.envValuePlaceholder")
        }
      />
      <Button
        size="icon"
        variant="ghost"
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        aria-label={t("services.envDelete")}
        onClick={() => onChange({ deleted: true })}
      >
        <Trash2 className="text-destructive" />
      </Button>
    </div>
  );
}

function FileDraftItem({
  row,
  error,
  disabled,
  permissionDescriptionID,
  onChange,
  onContent,
}: {
  row: SecretFileDraftRow;
  error?: "invalid" | "duplicate" | "content";
  disabled: boolean;
  permissionDescriptionID?: string;
  onChange: (update: Partial<SecretFileDraftRow>) => void;
  onContent: () => void;
}) {
  const { t } = useTranslations();
  if (row.deleted) {
    return (
      <StagedDeleteRow
        label={row.originalName}
        disabled={disabled}
        permissionDescriptionID={permissionDescriptionID}
        onUndo={() => onChange({ deleted: false })}
      />
    );
  }
  return (
    <div className="grid min-w-0 gap-2 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-start">
      <div className="min-w-0 space-y-1">
        <Input
          value={row.name}
          disabled={disabled}
          aria-describedby={disabled ? permissionDescriptionID : undefined}
          onChange={(event) => onChange({ name: event.target.value })}
          className="min-w-0 font-mono text-sm"
          aria-label={t("services.secretFileColName")}
          aria-invalid={Boolean(error)}
          placeholder={t("services.secretFileNamePlaceholder")}
        />
        {error ? (
          <p className="text-destructive text-xs" role="alert">
            {error === "duplicate"
              ? t("services.secretFileDuplicateName")
              : error === "content"
                ? t("services.secretFileContentRequired")
                : t("services.secretFileInvalidName")}
          </p>
        ) : null}
      </div>
      <Button
        variant="outline"
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        onClick={onContent}
      >
        <Pencil />
        {row.contentChanged
          ? t("services.secretFileEditContent")
          : t("services.secretFileViewContent")}
      </Button>
      <Button
        size="icon"
        variant="ghost"
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        aria-label={t("services.secretFileDelete")}
        onClick={() => onChange({ deleted: true })}
      >
        <Trash2 className="text-destructive" />
      </Button>
    </div>
  );
}

function AddVariableMenu({
  disabled,
  onAdd,
  onGenerate,
  onImport,
}: {
  disabled: boolean;
  onAdd: () => void;
  onGenerate: () => void;
  onImport: () => void;
}) {
  const { t } = useTranslations();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="sm" variant="outline" disabled={disabled}>
          <Plus /> {t("services.envAdd")} <ChevronDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={onAdd}>
          <Plus /> {t("services.envAddVariable")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onGenerate}>
          <Sparkles /> {t("services.envAddGenerated")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onImport}>
          <FileUp /> {t("services.envImport")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function EmptyCopy({ title, body }: { title: string; body: string }) {
  return (
    <div className="py-6 text-center">
      <p className="font-medium">{title}</p>
      <p className="text-muted-foreground mt-1 text-sm">{body}</p>
    </div>
  );
}

function RowsSkeleton() {
  return (
    <div className="space-y-3">
      {[0, 1, 2].map((index) => (
        <div key={index} className="flex gap-3">
          <Skeleton className="h-9 flex-1" />
          <Skeleton className="h-9 flex-1" />
        </div>
      ))}
    </div>
  );
}
