import { useMemo, useRef, useState } from "react";
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
import {
  classifyEnvVarError,
  useEnvVarKeys,
  useRevealEnvVar,
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
  isValidSecretFileName,
  validateEnvironmentDraft,
  type EnvDraftRow,
  type EnvironmentDraft,
  type SecretFileDraftRow,
} from "@/features/services/lib/environment-draft";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";
import {
  downloadEnvFile,
  formatEnvExport,
} from "@/features/services/lib/env-export";
import type { DotenvEntry } from "@/features/services/lib/dotenv-import";
import { EnvImportDialog } from "./env-import-dialog";
import { SecretFileContentDialog } from "./secret-file-content-dialog";

type SaveChoice = "only" | "deploy" | "rebuild";

export function ServiceEnvironmentEditor({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const env = useEnvVarKeys(serviceId);
  const files = useSecretFileNames(serviceId);
  const revealEnv = useRevealEnvVar(serviceId);
  const revealFile = useRevealSecretFile(serviceId);
  const { save, saving } = useEnvironmentDraftSave();
  const { trigger, deploying } = useTriggerDeploy();
  const nextID = useRef(0);
  const fileInput = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState<EnvironmentDraft | null>(null);
  const [revealedEnv, setRevealedEnv] = useState<Record<string, string>>({});
  const [revealedFiles, setRevealedFiles] = useState<Record<string, string>>(
    {},
  );
  const [revealing, setRevealing] = useState<string | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [contentRowID, setContentRowID] = useState<string | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState(false);
  const [savedPendingDeploy, setSavedPendingDeploy] = useState(false);
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
  const busy = saving || deploying;
  const blocker = useBlocker({
    shouldBlockFn: () => dirty,
    enableBeforeUnload: dirty,
    withResolver: true,
  });

  function beginEdit() {
    setDraft(
      createEnvironmentDraft(
        env.keys.map((entry) => entry.key),
        files.names.map((entry) => entry.name),
      ),
    );
    setSaveError(false);
    setUploadError(null);
  }

  function updateEnvRow(id: string, update: Partial<EnvDraftRow>) {
    setDraft((current) =>
      current
        ? {
            ...current,
            envVars: current.envVars.flatMap((row) =>
              row.id === id && row.originalKey == null && update.deleted
                ? []
                : [row.id === id ? { ...row, ...update } : row],
            ),
          }
        : current,
    );
  }

  function updateFileRow(id: string, update: Partial<SecretFileDraftRow>) {
    setDraft((current) =>
      current
        ? {
            ...current,
            secretFiles: current.secretFiles.flatMap((row) =>
              row.id === id && row.originalName == null && update.deleted
                ? []
                : [row.id === id ? { ...row, ...update } : row],
            ),
          }
        : current,
    );
  }

  function addVariable(generated = false) {
    if (!draft) return;
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
                value: generated ? generateEnvValue() : "",
                valueChanged: true,
                deleted: false,
              },
            ],
          }
        : current,
    );
  }

  function importVariables(entries: DotenvEntry[]) {
    setDraft((current) => {
      if (!current) return current;
      const rows = [...current.envVars];
      for (const entry of entries) {
        const index = rows.findIndex(
          (row) => !row.deleted && row.key.trim() === entry.key,
        );
        if (index >= 0) {
          rows[index] = {
            ...rows[index],
            value: entry.value,
            valueChanged: true,
          };
        } else {
          rows.push({
            id: `import-env:${nextID.current++}`,
            originalKey: null,
            key: entry.key,
            value: entry.value,
            valueChanged: true,
            deleted: false,
          });
        }
      }
      return { ...current, envVars: rows };
    });
  }

  function addSecretFile() {
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
    if (!selected || !draft) return;
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

  async function revealOne(kind: "env" | "file", name: string) {
    setRevealing(`${kind}:${name}`);
    try {
      const value =
        kind === "env" ? await revealEnv(name) : await revealFile(name);
      if (kind === "env") {
        setRevealedEnv((current) => ({ ...current, [name]: value }));
      } else {
        setRevealedFiles((current) => ({ ...current, [name]: value }));
      }
    } catch {
      toast.error(
        kind === "env"
          ? t("services.envRevealError")
          : t("services.secretFileRevealError"),
      );
    } finally {
      setRevealing(null);
    }
  }

  async function exportEnvironment(kind: "copy" | "download") {
    setExporting(true);
    try {
      const values = await Promise.all(
        env.keys.map(async ({ key }) => ({ key, value: await revealEnv(key) })),
      );
      const formatted = formatEnvExport(values);
      if (kind === "copy") {
        await navigator.clipboard.writeText(formatted);
        toast.success(t("services.envCopySuccess"));
      } else {
        downloadEnvFile(`${serviceId}.env`, formatted);
        toast.success(t("services.envExportSuccess"));
      }
    } catch {
      toast.error(t("services.envExportError"));
    } finally {
      setExporting(false);
    }
  }

  async function commit(choice: SaveChoice) {
    if (!draft || !dirty || !isDraftValid(validation)) return;
    setSaveError(false);
    try {
      const mode = choice === "deploy" ? "deploy" : "save_only";
      await save(serviceId, patch, mode);
    } catch {
      setSaveError(true);
      toast.error(t("services.environmentSaveError"));
      return;
    }

    // The configuration save is committed at this point. End the draft before
    // any refresh/deploy follow-up so a later failure can never cause a retry to
    // reapply an already-successful secret patch.
    setDraft(null);
    setRevealedEnv({});
    setRevealedFiles({});
    if (choice === "rebuild") {
      const deployID = await trigger(serviceId);
      if (!deployID) {
        setSavedPendingDeploy(true);
        return;
      }
    } else {
      toast.success(
        choice === "deploy"
          ? t("services.environmentSaveDeploySuccess")
          : t("services.environmentSaveOnlySuccess"),
      );
    }
    setSavedPendingDeploy(false);
  }

  async function retryDeploy() {
    const deployID = await trigger(serviceId);
    if (deployID) setSavedPendingDeploy(false);
  }

  const errorKind =
    classifyEnvVarError(env.error) ?? classifySecretFileError(files.error);
  const loading =
    (env.loading && env.keys.length === 0) ||
    (files.loading && files.names.length === 0);
  const contentRow = draft?.secretFiles.find((row) => row.id === contentRowID);

  return (
    <div className="space-y-6">
      {savedPendingDeploy ? (
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
              disabled={deploying}
              onClick={() => void retryDeploy()}
            >
              {deploying ? <Loader2 className="animate-spin" /> : <RotateCw />}
              {t("services.environmentRetryDeploy")}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {errorKind ? (
        <Alert variant="destructive">
          <AlertTitle>
            {t(
              `services.env${errorKind === "generic" ? "Error" : errorKind === "forbidden" ? "Forbidden" : "Unavailable"}Title`,
            )}
          </AlertTitle>
          <AlertDescription>
            {t(
              `services.env${errorKind === "generic" ? "Error" : errorKind === "forbidden" ? "Forbidden" : "Unavailable"}Body`,
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <ResponsiveCardHeader>
          <div>
            <CardTitle>{t("services.envTitle")}</CardTitle>
            <CardDescription className="mt-1.5">
              {t("services.envDescription")}
            </CardDescription>
          </div>
          <CardAction className="col-start-1 row-start-2 mt-2 justify-self-stretch sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
            <div className="flex flex-wrap gap-2 sm:justify-end">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={loading || Boolean(errorKind) || exporting}
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
              {!draft ? (
                <Button
                  size="sm"
                  disabled={loading || Boolean(errorKind)}
                  onClick={beginEdit}
                >
                  <Pencil /> {t("services.environmentEdit")}
                </Button>
              ) : (
                <AddVariableMenu
                  onAdd={() => addVariable(false)}
                  onGenerate={() => addVariable(true)}
                  onImport={() => setImportOpen(true)}
                />
              )}
            </div>
          </CardAction>
        </ResponsiveCardHeader>
        <CardContent>
          {loading ? (
            <RowsSkeleton />
          ) : draft ? (
            <div className="divide-y">
              {draft.envVars.map((row) => (
                <EnvDraftItem
                  key={row.id}
                  row={row}
                  error={validation.env[row.id]}
                  onChange={(update) => updateEnvRow(row.id, update)}
                />
              ))}
              {draft.envVars.length === 0 ? (
                <EmptyCopy
                  title={t("services.envEmptyTitle")}
                  body={t("services.envEmptyBody")}
                />
              ) : null}
            </div>
          ) : (
            <div className="divide-y">
              {env.keys.map(({ id, key }) => (
                <SensitiveViewItem
                  key={id}
                  name={key}
                  value={revealedEnv[key]}
                  loading={revealing === `env:${key}`}
                  onToggle={() =>
                    revealedEnv[key] !== undefined
                      ? setRevealedEnv((current) => {
                          const next = { ...current };
                          delete next[key];
                          return next;
                        })
                      : void revealOne("env", key)
                  }
                />
              ))}
              {env.keys.length === 0 ? (
                <EmptyCopy
                  title={t("services.envEmptyTitle")}
                  body={t("services.envEmptyBody")}
                />
              ) : null}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <ResponsiveCardHeader>
          <div>
            <CardTitle>{t("services.secretFilesTitle")}</CardTitle>
            <CardDescription className="mt-1.5">
              {t("services.secretFilesDescription")}
            </CardDescription>
          </div>
          {draft ? (
            <CardAction className="col-start-1 row-start-2 mt-2 justify-self-stretch sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
              <div className="flex flex-wrap gap-2 sm:justify-end">
                <Button size="sm" variant="outline" onClick={addSecretFile}>
                  <FilePlus2 /> {t("services.secretFileAdd")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => fileInput.current?.click()}
                >
                  <FileUp /> {t("services.secretFileUpload")}
                </Button>
                <input
                  ref={fileInput}
                  className="sr-only"
                  type="file"
                  multiple
                  onChange={(event) => {
                    void uploadFiles(event.target.files);
                    event.target.value = "";
                  }}
                />
              </div>
            </CardAction>
          ) : null}
        </ResponsiveCardHeader>
        <CardContent>
          {uploadError ? (
            <p className="text-destructive mb-3 text-sm" role="alert">
              {uploadError}
            </p>
          ) : null}
          {loading ? (
            <RowsSkeleton />
          ) : draft ? (
            <div className="divide-y">
              {draft.secretFiles.map((row) => (
                <FileDraftItem
                  key={row.id}
                  row={row}
                  error={validation.files[row.id]}
                  onChange={(update) => updateFileRow(row.id, update)}
                  onContent={() => setContentRowID(row.id)}
                />
              ))}
              {draft.secretFiles.length === 0 ? (
                <EmptyCopy
                  title={t("services.secretFilesEmptyTitle")}
                  body={t("services.secretFilesEmptyBody")}
                />
              ) : null}
            </div>
          ) : (
            <div className="divide-y">
              {files.names.map(({ id, name }) => (
                <SensitiveViewItem
                  key={id}
                  name={name}
                  value={revealedFiles[name]}
                  loading={revealing === `file:${name}`}
                  onToggle={() =>
                    revealedFiles[name] !== undefined
                      ? setRevealedFiles((current) => {
                          const next = { ...current };
                          delete next[name];
                          return next;
                        })
                      : void revealOne("file", name)
                  }
                />
              ))}
              {files.names.length === 0 ? (
                <EmptyCopy
                  title={t("services.secretFilesEmptyTitle")}
                  body={t("services.secretFilesEmptyBody")}
                />
              ) : null}
            </div>
          )}
        </CardContent>
      </Card>

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
              <div className="flex">
                <Button
                  className="flex-1 rounded-r-none sm:flex-none"
                  disabled={!dirty || !isDraftValid(validation) || busy}
                  onClick={() => void commit("deploy")}
                >
                  {busy ? <Loader2 className="animate-spin" /> : <Check />}
                  {t("services.environmentSaveDeploy")}
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      className="rounded-l-none border-l border-primary-foreground/30 px-2"
                      disabled={!dirty || !isDraftValid(validation) || busy}
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
            </div>
          </CardContent>
        </Card>
      ) : null}

      <EnvImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImport={importVariables}
      />
      {contentRow ? (
        <SecretFileContentDialog
          open
          name={contentRow.name || t("services.secretFileUntitled")}
          content={contentRow.content}
          reveal={
            contentRow.originalName
              ? () => revealFile(contentRow.originalName as string)
              : undefined
          }
          onOpenChange={(open) => {
            if (!open) setContentRowID(null);
          }}
          onSave={(content, changed) =>
            updateFileRow(contentRow.id, {
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

function ResponsiveCardHeader({ children }: { children: React.ReactNode }) {
  return (
    <CardHeader className="grid-cols-1 grid-rows-none sm:grid-cols-[minmax(0,1fr)_auto] sm:grid-rows-[auto_auto]">
      {children}
    </CardHeader>
  );
}

function SensitiveViewItem({
  name,
  value,
  loading,
  onToggle,
}: {
  name: string;
  value: string | undefined;
  loading: boolean;
  onToggle: () => void;
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
        {visible ? value : "••••••••••••"}
      </code>
      <Button size="sm" variant="ghost" disabled={loading} onClick={onToggle}>
        {loading ? (
          <Loader2 className="animate-spin" />
        ) : visible ? (
          <EyeOff />
        ) : (
          <Eye />
        )}
        {visible ? t("services.envHide") : t("services.envReveal")}
      </Button>
    </div>
  );
}

function EnvDraftItem({
  row,
  error,
  onChange,
}: {
  row: EnvDraftRow;
  error?: "invalid" | "duplicate" | "value";
  onChange: (update: Partial<EnvDraftRow>) => void;
}) {
  const { t } = useTranslations();
  if (row.deleted) {
    return (
      <div className="flex min-w-0 items-center justify-between gap-3 py-4 first:pt-0 last:pb-0">
        <code className="min-w-0 break-all text-sm line-through opacity-60">
          {row.originalKey}
        </code>
        <div className="flex items-center gap-2">
          <Badge variant="outline">
            {t("services.environmentStagedDelete")}
          </Badge>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => onChange({ deleted: false })}
          >
            <Undo2 /> {t("services.environmentUndo")}
          </Button>
        </div>
      </div>
    );
  }
  return (
    <div className="grid min-w-0 gap-2 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start">
      <div className="min-w-0 space-y-1">
        <Input
          value={row.key}
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
        onChange={(event) =>
          onChange({ value: event.target.value, valueChanged: true })
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
  onChange,
  onContent,
}: {
  row: SecretFileDraftRow;
  error?: "invalid" | "duplicate" | "content";
  onChange: (update: Partial<SecretFileDraftRow>) => void;
  onContent: () => void;
}) {
  const { t } = useTranslations();
  if (row.deleted) {
    return (
      <div className="flex min-w-0 items-center justify-between gap-3 py-4 first:pt-0 last:pb-0">
        <code className="min-w-0 break-all text-sm line-through opacity-60">
          {row.originalName}
        </code>
        <div className="flex items-center gap-2">
          <Badge variant="outline">
            {t("services.environmentStagedDelete")}
          </Badge>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => onChange({ deleted: false })}
          >
            <Undo2 /> {t("services.environmentUndo")}
          </Button>
        </div>
      </div>
    );
  }
  return (
    <div className="grid min-w-0 gap-2 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-start">
      <div className="min-w-0 space-y-1">
        <Input
          value={row.name}
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
      <Button variant="outline" onClick={onContent}>
        <Pencil />
        {row.contentChanged
          ? t("services.secretFileEditContent")
          : t("services.secretFileViewContent")}
      </Button>
      <Button
        size="icon"
        variant="ghost"
        aria-label={t("services.secretFileDelete")}
        onClick={() => onChange({ deleted: true })}
      >
        <Trash2 className="text-destructive" />
      </Button>
    </div>
  );
}

function AddVariableMenu({
  onAdd,
  onGenerate,
  onImport,
}: {
  onAdd: () => void;
  onGenerate: () => void;
  onImport: () => void;
}) {
  const { t } = useTranslations();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="sm" variant="outline">
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
    <div className="py-8 text-center">
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
