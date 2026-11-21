import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useBlocker } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  AlertTriangle,
  Check,
  ChevronDown,
  Copy,
  Download,
  Eye,
  EyeOff,
  FileCode2,
  FilePlus2,
  FileUp,
  Loader2,
  Pencil,
  Plus,
  RotateCw,
  ShieldAlert,
  Sparkles,
  Trash2,
  Undo2,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { PanelCenteredState } from "@/common/components/panel-states";
import { SectionErrorNotice } from "@/features/services/components/section-error-notice";
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
import { AutoTextarea } from "@/common/components/ui/auto-textarea";
import { Badge } from "@/common/components/ui/badge";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  ENV_CARD_ACTION_CLASS,
  ENV_CARD_DESCRIPTION_CLASS,
  ENV_CARD_HEADER_CLASS,
  ENV_LIST_CLASS,
  ENV_LIST_FULL_ROW_CLASS,
  ENV_ROW_ACTIONS_CLASS,
  ENV_ROW_CLASS,
  ENV_ROW_NAME_CLASS,
  ENV_ROW_VALUE_CLASS,
  EnvListHead,
  EnvRowsSkeleton,
} from "@/features/services/components/env-rows-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
  type RefusalKind,
} from "@/common/lib/graphql-error";
import { useReauthDraft } from "@/common/hooks/use-reauth-draft";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  useEnvVarKeys,
  useRevealEnvVar,
} from "@/features/services/hooks/use-env-vars";
import {
  useRevealSecretFile,
  useSecretFileNames,
} from "@/features/services/hooks/use-secret-files";
import { useEnvironmentDraftSave } from "@/features/services/hooks/use-environment-draft-save";
import { classifySecretStoreError } from "@/features/services/lib/secret-store";
import {
  MAX_SECRET_FILE_BYTES,
  NO_ENVIRONMENT_EDIT,
  admitUploadedFiles,
  type SkippedUpload,
  createEnvironmentDraft,
  editDraft,
  endEdit,
  environmentDraftPatch,
  isDraftValid,
  startEdit,
  type DraftValidation,
  isNewDraftRow,
  isValidSecretFileName,
  MASKED_VALUE,
  reidentifyDraft,
  validateEnvironmentDraft,
  type EnvDraftRow,
  type EnvironmentDraft,
  type SecretFileDraftRow,
  type EnvironmentPatchInput,
} from "@/features/services/lib/environment-draft";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";
import { isManifestManaged } from "@/features/services/types";
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
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";

type SaveChoice = "only" | "deploy" | "rebuild";

// Spelled out rather than assembled from the kind, so every key is greppable
// and a new RefusalKind fails to compile instead of rendering its own key.
const ENV_ERROR_COPY: Record<RefusalKind, { title: string; body: string }> = {
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
// The Secret Files card says what failed in its own words: a refused or
// unconfigured store names files, not variables.
const FILES_ERROR_COPY: Record<RefusalKind, { title: string; body: string }> = {
  generic: {
    title: "services.secretFilesErrorTitle",
    body: "services.secretFilesErrorBody",
  },
  forbidden: {
    title: "services.envForbiddenTitle",
    body: "services.secretFilesForbiddenBody",
  },
  unavailable: {
    title: "services.secretFilesUnavailableTitle",
    body: "services.envUnavailableBody",
  },
};

export function ServiceEnvironmentEditor({
  serviceId,
  suspended = false,
  dockerBuild = false,
}: {
  serviceId: string;
  /** From the page's cached service read, so no second deploy-actions query. */
  suspended?: boolean;
  /**
   * The service builds from a Dockerfile (isDockerBuild). A Docker build also
   * gets each secret file in the root of its build context, as on Render,
   * where `COPY . .` bakes it into the image; the file dialog says so and
   * names .dockerignore (w8/253).
   */
  dockerBuild?: boolean;
}) {
  const env = useEnvVarKeys(serviceId);
  const files = useSecretFileNames(serviceId);
  const revealEnv = useRevealEnvVar(serviceId);
  const revealFile = useRevealSecretFile(serviceId);
  const { save, saving } = useEnvironmentDraftSave();
  const { t } = useTranslations();
  // The rebuild's outcome toasts as a Manual Deploy's did: a rebuild that did
  // not start keeps the save and says why (w4/347).
  const rebuild = async (patch: EnvironmentPatchInput) => {
    const result = await save(serviceId, patch, "rebuild");
    if (result.rolledOut) toast.success(t("services.triggerDeploySuccess"));
    else toast.error(result.rolloutError ?? t("services.triggerDeployError"));
    return result;
  };

  return (
    <EnvironmentEditor
      // A draft belongs to one service: another service starts a fresh editor.
      key={serviceId}
      // The tab's name is the layout's h2; its cards sit under it.
      headingLevel="h3"
      resourceId={serviceId}
      copy={
        dockerBuild
          ? {
              secretFileDialogDescription: t(
                "services.secretFileContentDialogDescriptionDocker",
              ),
            }
          : undefined
      }
      // A suspended service deploys nothing: the deploying save modes wait for
      // Resume, as Manual Deploy does (w4/m234).
      deployBlockedReason={
        suspended ? t("capabilities.blockedSuspended") : undefined
      }
      envKeys={env.keys}
      // While the keys read is in flight the manifest flag may not have
      // arrived yet; the editor fails closed on any row missing it rather than
      // flashing a manifest-owned variable as editable (w4/m120 t003).
      envKeysPending={env.loading}
      secretFileNames={files.names}
      loading={
        (env.loading && env.keys.length === 0) ||
        (files.loading && files.names.length === 0)
      }
      errorKind={
        classifySecretStoreError(env.error) ??
        classifySecretStoreError(files.error)
      }
      // Both lists read again; a refusal that persists keeps its state.
      onRetry={() => {
        void env.refetch().catch(() => undefined);
        void files.refetch().catch(() => undefined);
      }}
      revealEnv={revealEnv}
      revealFile={revealFile}
      saving={saving}
      save={async (patch, choice) => {
        if (choice === "rebuild") {
          // One call saves and rebuilds, so the deploy reads as a
          // configuration change (w4/347). It always rebuilds once.
          const { refreshFailed, rolledOut } = await rebuild(patch);
          return {
            affectedServiceIds: [serviceId],
            rolloutFailed: !rolledOut,
            refreshFailed,
          };
        }
        const { refreshFailed, rolledOut } = await save(
          serviceId,
          patch,
          choice === "deploy" ? "deploy" : "save_only",
        );
        // A deploying save that changed nothing rolls nothing out; saying a
        // deployment started would be false (w4/216).
        return {
          affectedServiceIds:
            choice === "deploy" && rolledOut ? [serviceId] : [],
          refreshFailed,
        };
      }}
      // Only a rebuild reports a failed rollout. Its retry re-sends an empty
      // rebuild patch, as an environment group's does, keeping the cause.
      retryRollout={async () =>
        (await rebuild({ envVars: [], secretFiles: [] })).rolledOut
      }
    />
  );
}

export interface EnvironmentEditorCopy {
  envTitle?: string;
  envDescription?: string;
  envEmptyTitle?: string;
  envEmptyBody?: string;
  secretFilesTitle?: string;
  secretFilesDescription?: string;
  secretFilesEmptyTitle?: string;
  secretFilesEmptyBody?: string;
  /** The secret-file dialog's sentence on where the file is available. */
  secretFileDialogDescription?: string;
}

export interface EnvironmentEditorProps {
  resourceId: string;
  envKeys: Array<{ id: string; key: string; managedBy?: string | null }>;
  /**
   * The env-key read is still in flight, so `managedBy` may not have arrived on
   * every row yet. Rows missing the flag are treated as read-only until it
   * settles (w4/m120 t003) — a manifest variable must never appear editable,
   * not even for one frame.
   */
  envKeysPending?: boolean;
  secretFileNames: Array<{ id: string; name: string }>;
  loading: boolean;
  errorKind: RefusalKind | null;
  /** Read the lists again after a failed read; offered beside the error. */
  onRetry?: () => void;
  revealEnv: (key: string) => Promise<string>;
  revealFile: (name: string) => Promise<string>;
  /**
   * The resource's latest read revision, for a revision-aware resource (an
   * environment group, w4/m161). Edit captures it into the draft as
   * `baseRevision`, and only that captured value reaches `save` — later polls
   * update this prop without moving an open draft's base. `null` means the
   * revision has not been read, so Edit stays unavailable. Leave unset for a
   * service, whose save carries no revision.
   */
  revision?: string | null;
  save: (
    patch: EnvironmentPatchInput,
    choice: SaveChoice,
    baseRevision: string | undefined,
  ) => Promise<{
    affectedServiceIds?: readonly string[];
    failedServiceIds?: readonly string[];
    rolloutFailed?: boolean;
    refreshFailed?: boolean;
  }>;
  retryRollout: (choice: Exclude<SaveChoice, "only">) => Promise<boolean>;
  saving: boolean;
  generateOnServer?: boolean;
  /** Override section copy when the editor is hosted outside a service (w4/067). */
  copy?: EnvironmentEditorCopy;
  /**
   * The cards' heading level: h2 under a page's h1 (an environment group),
   * h3 under a service tab's own h2.
   */
  headingLevel?: "h2" | "h3";
  /**
   * Why the deploying save modes cannot run now — a suspended service
   * (w4/m234). Save only becomes the primary action, and the two deploying
   * items stay disabled with this reason.
   */
  deployBlockedReason?: string;
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
  envKeysPending = false,
  secretFileNames,
  loading,
  errorKind,
  onRetry,
  revealEnv,
  revealFile,
  revision,
  save,
  retryRollout,
  saving,
  generateOnServer = false,
  copy,
  headingLevel = "h2",
  deployBlockedReason,
}: EnvironmentEditorProps) {
  const { t } = useTranslations();
  const envTitle = copy?.envTitle ?? t("services.envTitle");
  const envDescription = copy?.envDescription ?? t("services.envDescription");
  const envEmptyTitle = copy?.envEmptyTitle ?? t("services.envEmptyTitle");
  const envEmptyBody = copy?.envEmptyBody ?? t("services.envEmptyBody");
  const secretFilesTitle =
    copy?.secretFilesTitle ?? t("services.secretFilesTitle");
  const secretFilesDescription =
    copy?.secretFilesDescription ?? t("services.secretFilesDescription");
  const secretFilesEmptyTitle =
    copy?.secretFilesEmptyTitle ?? t("services.secretFilesEmptyTitle");
  const secretFilesEmptyBody =
    copy?.secretFilesEmptyBody ?? t("services.secretFilesEmptyBody");
  // The keys the dashboard must not write. A manifest-owned variable is one
  // (bex-api refuses every write to it with ENV_VAR_MANIFEST_MANAGED); so is a
  // row whose flag has not arrived yet, because rendering it editable and
  // taking it away a frame later is exactly the flash t003 forbids.
  const readOnlyEnvKeys = useMemo(
    () =>
      new Set(
        envKeys
          .filter(
            ({ managedBy }) =>
              isManifestManaged(managedBy) ||
              (managedBy === undefined && envKeysPending),
          )
          .map(({ key }) => key),
      ),
    [envKeys, envKeysPending],
  );
  const capabilities = useCapabilities();
  const { canCreate, canViewSensitive } = capabilities;
  const createDenied = !canCreate;
  const sensitiveDenied = !canViewSensitive;
  const createReasonKey = capabilities.reasonKey("can_create");
  const sensitiveReasonKey = capabilities.reasonKey("can_view_sensitive");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const sensitiveReason = sensitiveReasonKey
    ? t(sensitiveReasonKey)
    : undefined;
  const writeReasonID = useId();
  const nextID = useRef(0);
  const fileInput = useRef<HTMLInputElement>(null);
  const [editor, setEditor] = useState(NO_ENVIRONMENT_EDIT);
  const draft = editor.draft;
  const setDraft = (
    update: (draft: EnvironmentDraft | null) => EnvironmentDraft | null,
  ) => setEditor((state) => editDraft(state, update));
  // Preserve an in-progress draft across a session-expiry redirect (w3/m80
  // t005): if the session lapses mid-edit, the Apollo auth link hard-navigates
  // to login and this component unmounts — the draft is mirrored to
  // sessionStorage so sign-in returns the user to their exact unsaved edits.
  // Destructured so the stable callbacks (not the fresh wrapper object) drive
  // the effects below.
  const {
    save: persistDraft,
    clear: clearPersistedDraft,
    consumeRestored,
  } = useReauthDraft<EnvironmentDraft>(`env:${resourceId}`);
  const [restoredDraft, setRestoredDraft] = useState(false);
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
  const dirty = patch.envVars.length > 0 || patch.secretFiles.length > 0;
  const busy = saving;
  // Menu items mirror their trigger's disabled state; see AddVariableMenu.
  // Nothing to export is not an export: an empty list disables it too.
  const exportDisabled =
    loading ||
    Boolean(errorKind) ||
    exporting ||
    sensitiveDenied ||
    envKeys.length === 0;
  const saveDisabled =
    !dirty || !isDraftValid(validation) || busy || createDenied;
  const blocker = useBlocker({
    shouldBlockFn: () => dirty,
    enableBeforeUnload: dirty,
    withResolver: true,
  });

  // Restore a draft this tab left unsaved, by a reload or a session expiry
  // (the banner names neither, w4/337) — once, in an effect
  // (not a render initializer) so the client's first render still matches the
  // draft-less SSR output and there's no hydration mismatch.
  useEffect(() => {
    const saved = consumeRestored();
    if (!saved) return;
    // Re-key through THIS mount's allocator before adopting. The stored ids
    // were allocated by the previous mount, whose counter is gone; reinstating
    // them verbatim let the next added row collide with a restored one, and
    // `updateRow` acts on every row whose id matches — so one edit rewrote two
    // rows and one delete removed two (w4/139). Reading the ref here is safe:
    // this is an effect, not render.
    const fresh = reidentifyDraft(saved, () => nextID.current++);
    setEditor((state) => (state.draft ? state : startEdit(state, fresh)));
    setRestoredDraft(true);
  }, [consumeRestored]);

  // Mirror the live draft to sessionStorage while it holds unsaved changes, and
  // drop it the moment there's nothing to preserve (saved, cancelled, or clean).
  // Reuses the render-scope `dirty` rather than rebuilding the patch here.
  useEffect(() => {
    if (draft && dirty) persistDraft(draft);
    else clearPersistedDraft();
  }, [draft, dirty, persistDraft, clearPersistedDraft]);

  const editUnavailable =
    loading || Boolean(errorKind) || createDenied || revision === null;

  function beginEdit() {
    if (editUnavailable) return;
    const fresh = createEnvironmentDraft(
      envKeys.map((entry) => entry.key),
      secretFileNames.map((entry) => entry.name),
      readOnlyEnvKeys,
    );
    // Captured with the names snapshot, in the same click: the draft's base is
    // the state the user saw when they chose Edit, never a later poll (w4/m161).
    setEditor((state) =>
      startEdit(state, revision ? { ...fresh, baseRevision: revision } : fresh),
    );
    setSaveError(false);
    setRestoredDraft(false); // a fresh edit, not one recovered from a redirect
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
              row.id !== id ||
              // A read-only row ignores every update, whatever called this —
              // a stale handler, a keyboard path, a future control.
              ("readOnly" in row && row.readOnly === true)
                ? [row]
                : isNewDraftRow(row) && update.deleted
                  ? []
                  : [{ ...row, ...update }],
            ),
          }
        : current,
    );
  }

  // `entering`: called in the same click as beginEdit, before the draft it
  // starts has rendered, so the row is added to whatever draft is current.
  function addVariable(generated = false, entering = false) {
    if ((!draft && !entering) || createDenied) return;
    const id = `new-env:${nextID.current++}`;
    const value = generated && !generateOnServer ? generateEnvValue() : "";
    setDraft((current) => {
      if (!current) return current;
      // Trimmed, as validation and the save patch compare keys: a padded
      // " NEW_SECRET " already holds the name (w4/286).
      const existing = new Set(current.envVars.map((row) => row.key.trim()));
      let key = generated ? "NEW_SECRET" : "";
      let suffix = 2;
      while (key && existing.has(key)) key = `NEW_SECRET_${suffix++}`;
      return {
        ...current,
        envVars: [
          ...current.envVars,
          {
            id,
            originalKey: null,
            key,
            value,
            valueChanged: true,
            generateValue: generated && generateOnServer,
            deleted: false,
          },
        ],
      };
    });
  }

  function importVariables(entries: DotenvEntry[]) {
    if (createDenied) return;
    // A .env file naming a manifest-owned key must not smuggle in the write
    // the row's missing controls refuse.
    const writable = entries.filter((entry) => !readOnlyEnvKeys.has(entry.key));
    setDraft((current) => {
      if (!current) return current;
      return {
        ...current,
        envVars: upsertDotenvEntries(
          current.envVars,
          writable,
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
    const { session } = editor;
    const decoded: SecretFileDraftRow[] = [];
    const skipped: SkippedUpload[] = [];
    for (const file of Array.from(selected)) {
      const name = file.name.split(/[\\/]/).at(-1) ?? "";
      if (!isValidSecretFileName(name)) {
        skipped.push({ fileName: name || file.name, reason: "name" });
        continue;
      }
      if (file.size > MAX_SECRET_FILE_BYTES) {
        skipped.push({ fileName: name, reason: "size" });
        continue;
      }
      try {
        // Secret files travel as JSON strings, so only UTF-8 text survives:
        // file.text() would silently swap every invalid byte for U+FFFD and
        // save a mangled keystore (w8/029). A strict decode refuses it.
        // ignoreBOM keeps a leading byte-order mark as content: the default
        // drops it, so the stored file would differ from the one selected
        // (w4/285).
        const content = new TextDecoder("utf-8", {
          fatal: true,
          ignoreBOM: true,
        }).decode(await file.arrayBuffer());
        if (content.includes("\0")) {
          skipped.push({ fileName: name, reason: "encoding" });
          continue;
        }
        decoded.push({
          id: `upload-file:${nextID.current++}`,
          originalName: null,
          name,
          content,
          contentChanged: true,
          deleted: false,
        });
      } catch {
        skipped.push({ fileName: name, reason: "encoding" });
      }
    }
    setEditor((state) => admitUploadedFiles(state, session, decoded, skipped));
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
    // A revision-aware draft with no base was stored before drafts carried one.
    // Sending it with today's revision would overwrite whatever changed since,
    // and sending none would be an unchecked write — so it stays visible but
    // cannot save until the user discards it and edits afresh (w4/m161).
    if (revision !== undefined && !draft.baseRevision) {
      setSaveError(true);
      toast.error(t("services.environmentDraftStale"));
      return;
    }
    let result: Awaited<ReturnType<EnvironmentEditorProps["save"]>>;
    try {
      result = await save(patch, choice, draft.baseRevision);
    } catch (err) {
      // The draft and its base survive a refusal. A revision conflict is never
      // retried with a fresher token: the user decides what to keep.
      setSaveError(true);
      toast.error(
        hasGraphQLErrorCode(err, "ENV_GROUP_REVISION_CONFLICT")
          ? t("services.environmentDraftStale")
          : mutationErrorMessage(err, t("services.environmentSaveError")),
      );
      return;
    }

    // The save committed even if a follow-up read or deploy failed. Retrying
    // that follow-up must not reapply the accepted secret patch.
    setEditor(endEdit);
    reveals.clear();
    if (result.refreshFailed) {
      toast.warning(t("services.environmentSavedRefreshFailed"));
    }
    const rolloutFailed =
      result.rolloutFailed || Boolean(result.failedServiceIds?.length);
    if (choice !== "only" && rolloutFailed) {
      setPendingChoice(choice);
      return;
    }
    if (choice !== "rebuild" && !result.refreshFailed) {
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
    // retryRollout may reject (e.g. a dropped connection); say so and keep
    // the retry button in place.
    try {
      if (await retryRollout(pendingChoice)) {
        setPendingChoice(null);
      }
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("services.triggerDeployError")));
    }
  }

  const contentRow = draft?.secretFiles.find((row) => row.id === contentRowID);

  async function copyFresh(kind: "env" | "file", name: string) {
    if (sensitiveDenied) return;
    try {
      const value = await (kind === "env" ? revealEnv(name) : revealFile(name));
      await navigator.clipboard.writeText(value);
      // Named after the one value that moved — the bulk envCopySuccess toast
      // belongs to exportEnvironment's full dotenv export only (w6/044).
      toast.success(t("services.envCopyOneSuccess", { name }));
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

      {restoredDraft && draft ? (
        <Alert>
          <Undo2 />
          <AlertTitle>{t("services.environmentRestoredTitle")}</AlertTitle>
          <AlertDescription>
            <p>{t("services.environmentRestoredBody")}</p>
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                setEditor(endEdit);
                setRestoredDraft(false);
              }}
            >
              {t("services.environmentRestoredDiscard")}
            </Button>
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
        headingLevel={headingLevel}
        title={envTitle}
        description={envDescription}
        empty={{
          title: envEmptyTitle,
          body: envEmptyBody,
          // Read mode's way in, as the Secret Files card's empty state has:
          // Edit with one blank row already added.
          action: !draft ? (
            <PermissionTooltip reason={createReason}>
              <Button
                size="sm"
                disabled={editUnavailable}
                onClick={() => {
                  beginEdit();
                  addVariable(false, true);
                }}
              >
                <Plus /> {t("services.envAddVariable")}
              </Button>
            </PermissionTooltip>
          ) : undefined,
        }}
        error={
          errorKind
            ? {
                kind: errorKind,
                title: t(ENV_ERROR_COPY[errorKind].title),
                body: t(ENV_ERROR_COPY[errorKind].body),
                onRetry,
              }
            : undefined
        }
        head={{
          name: t("services.envColKey"),
          value: t("services.envColValue"),
        }}
        loading={loading}
        action={
          <>
            <DropdownMenu>
              <PermissionTooltip reason={sensitiveReason}>
                <DropdownMenuTrigger asChild>
                  <Button size="sm" variant="outline" disabled={exportDisabled}>
                    {exporting ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <Download />
                    )}
                    {t("services.envExport")}
                    <ChevronDown />
                  </Button>
                </DropdownMenuTrigger>
              </PermissionTooltip>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  disabled={exportDisabled}
                  onSelect={() => void exportEnvironment("copy")}
                >
                  <Copy /> {t("services.envCopy")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={exportDisabled}
                  onSelect={() => void exportEnvironment("download")}
                >
                  <Download /> {t("services.envDownload")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            {!draft ? (
              <PermissionTooltip reason={createReason}>
                <Button
                  size="sm"
                  // Neutral, as Render's is: the header's Manual Deploy is the
                  // screen's filled action, and while editing Save is. With
                  // no variables the empty state's Add variable is filled.
                  variant="outline"
                  disabled={editUnavailable}
                  onClick={beginEdit}
                >
                  <Pencil /> {t("services.environmentEdit")}
                </Button>
              </PermissionTooltip>
            ) : (
              <AddVariableMenu
                disabled={createDenied}
                reason={createReason}
                onAdd={() => addVariable(false)}
                onGenerate={() => addVariable(true)}
                onImport={() => setImportOpen(true)}
              />
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
                readOnlyNote={readOnlyEnvKeys.has(key)}
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
        headingLevel={headingLevel}
        skeletonRows={1}
        title={secretFilesTitle}
        description={secretFilesDescription}
        empty={{
          title: secretFilesEmptyTitle,
          body: secretFilesEmptyBody,
          // In read mode the panel's only enabling control (Edit) lives under
          // the Environment Variables header — so the empty state carries its
          // own affordance that enters the draft with a blank file row (w6/057).
          action: !draft ? (
            <PermissionTooltip reason={createReason}>
              <Button
                size="sm"
                variant="outline"
                disabled={editUnavailable}
                onClick={() => {
                  beginEdit();
                  addSecretFile();
                }}
              >
                <FilePlus2 /> {t("services.secretFileAdd")}
              </Button>
            </PermissionTooltip>
          ) : undefined,
        }}
        error={
          errorKind
            ? {
                kind: errorKind,
                title: t(FILES_ERROR_COPY[errorKind].title),
                // The variables card's Try again reloads both lists; a second
                // button here would retry the same thing (generic failures).
                body:
                  errorKind === "generic"
                    ? t("services.secretFilesReloadWithVariables")
                    : t(FILES_ERROR_COPY[errorKind].body),
                compact: true,
              }
            : undefined
        }
        head={{
          name: t("services.secretFileColName"),
          value: t("services.secretFileColContent"),
        }}
        loading={loading}
        notice={
          editor.uploadSkipped.length ? (
            <SkippedUploadsNotice skipped={editor.uploadSkipped} />
          ) : null
        }
        action={
          // Read mode: the card names its own way to add a file, as Render's
          // does, instead of leaving it behind the Variables card's Edit. An
          // empty list's state carries the same control, so it shows here
          // only beside existing files.
          !draft && (loading || secretFileNames.length > 0) && !errorKind ? (
            <PermissionTooltip reason={createReason}>
              <Button
                size="sm"
                variant="outline"
                disabled={editUnavailable}
                onClick={() => {
                  beginEdit();
                  addSecretFile();
                }}
              >
                <FilePlus2 /> {t("services.secretFileAdd")}
              </Button>
            </PermissionTooltip>
          ) : draft ? (
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
                hidden
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
        <Card className="sticky bottom-3 z-20 gap-3 py-3 shadow-lg @2xl/page:py-4">
          <CardContent className="flex flex-col gap-3 @2xl/page:flex-row @2xl/page:items-center @2xl/page:justify-between">
            <div className="min-w-0">
              {/* Until something changes the bar says the draft is open and
                  empty; it never claims unsaved changes it does not hold. */}
              {/* On a narrow page the status line under it says enough; the
                  title stays for screen readers and the bar keeps one line. */}
              <p className="font-medium @max-2xl/page:sr-only">
                {t(
                  dirty
                    ? "services.environmentUnsavedTitle"
                    : "services.environmentEditingTitle",
                )}
              </p>
              <p
                className="text-muted-foreground text-sm max-md:text-base"
                aria-live="polite"
              >
                {/* Two counts, two plural decisions — one i18next `count` can
                    only drive one, so the sentence is two pluralized halves
                    composed here (w6/062). A half with nothing in it is left
                    out. */}
                {dirty
                  ? [
                      patch.envVars.length
                        ? t("services.environmentUnsavedVariables", {
                            count: patch.envVars.length,
                          })
                        : null,
                      patch.secretFiles.length
                        ? t("services.environmentUnsavedFiles", {
                            count: patch.secretFiles.length,
                          })
                        : null,
                    ]
                      .filter(Boolean)
                      .join(" · ")
                  : t("services.environmentNoChanges")}
              </p>
              {saveError ? (
                <p className="text-destructive text-sm" role="alert">
                  {t("services.environmentSaveError")}
                </p>
              ) : null}
            </div>
            {/* One row at every width: on a phone the two stacked buttons
                took a fifth of the screen from the rows being edited. */}
            <div className="flex gap-2">
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => setEditor(endEdit)}
              >
                {t("services.envCancel")}
              </Button>
              <DropdownMenu>
                <PermissionTooltip reason={createReason}>
                  <div className="flex flex-1 @2xl/page:flex-none">
                    <Button
                      className="flex-1 rounded-r-none @2xl/page:flex-none"
                      disabled={saveDisabled}
                      onClick={() =>
                        void commit(deployBlockedReason ? "only" : "deploy")
                      }
                    >
                      {busy ? <Loader2 className="animate-spin" /> : <Check />}
                      {t(
                        deployBlockedReason
                          ? "services.environmentSaveOnly"
                          : "services.environmentSaveDeploy",
                      )}
                    </Button>
                    <DropdownMenuTrigger asChild>
                      <Button
                        className="rounded-l-none border-l border-primary-foreground/30 px-2"
                        disabled={saveDisabled}
                        aria-label={t("services.environmentSaveOptions")}
                      >
                        <ChevronDown />
                      </Button>
                    </DropdownMenuTrigger>
                  </div>
                </PermissionTooltip>
                <DropdownMenuContent align="end">
                  <PermissionMenuItem
                    disabled={saveDisabled}
                    permissionReason={deployBlockedReason}
                    onSelect={() => void commit("rebuild")}
                  >
                    {t("services.environmentSaveRebuild")}
                  </PermissionMenuItem>
                  <PermissionMenuItem
                    disabled={saveDisabled}
                    permissionReason={deployBlockedReason}
                    onSelect={() => void commit("deploy")}
                  >
                    {t("services.environmentSaveDeploy")}
                  </PermissionMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    disabled={saveDisabled}
                    onSelect={() => void commit("only")}
                  >
                    {t("services.environmentSaveOnly")}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
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
          description={copy?.secretFileDialogDescription}
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

      <ConfirmDialog
        open={blocker.status === "blocked"}
        title={t("services.environmentDiscardTitle")}
        description={t("services.environmentDiscardBody")}
        cancelLabel={t("services.environmentKeepEditing")}
        confirmLabel={t("services.environmentDiscard")}
        // Cancel must RESET the navigation blocker, not just close the dialog —
        // leaving it blocked would strand the user on the page.
        onCancel={() => blocker.reset?.()}
        onConfirm={() => {
          setEditor(endEdit);
          blocker.proceed?.();
        }}
      />
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
  headingLevel,
  skeletonRows = 2,
  title,
  description,
  action,
  notice,
  loading,
  error,
  head,
  isEmpty,
  empty,
  children,
}: {
  headingLevel: "h2" | "h3";
  /** Rows the loading frame draws: the ready list's usual length. */
  skeletonRows?: number;
  title: string;
  description: string;
  action?: React.ReactNode;
  notice?: React.ReactNode;
  loading: boolean;
  /**
   * A failed read: the card says so in place of its list, with a retry,
   * instead of an empty list that would read as "nothing is configured".
   */
  error?: {
    kind: RefusalKind;
    title: string;
    body: string;
    onRetry?: () => void;
    /** A one-line notice instead of the centred panel. */
    compact?: boolean;
  };
  /** The column labels, shown over the rows on a wide page. */
  head: { name: string; value: string };
  isEmpty: boolean;
  empty: { title: string; body: string; action?: React.ReactNode };
  children: React.ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader className={ENV_CARD_HEADER_CLASS}>
        <div>
          <CardTitle as={headingLevel}>{title}</CardTitle>
          <CardDescription className={ENV_CARD_DESCRIPTION_CLASS}>
            {description}
          </CardDescription>
        </div>
        {action ? (
          <CardAction className={ENV_CARD_ACTION_CLASS}>
            <div className="flex flex-wrap gap-2 @2xl/page:justify-end">
              {action}
            </div>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent>
        {notice ? (
          <div className="text-destructive mb-3 text-sm" role="alert">
            {notice}
          </div>
        ) : null}
        {loading ? (
          <EnvRowsSkeleton rows={skeletonRows} />
        ) : error && isEmpty && !error.compact ? (
          // Nothing loaded: the card says what failed in place of a list,
          // never "No environment variables".
          <div role="alert">
            <PanelCenteredState
              icon={
                error.kind === "forbidden" ? <ShieldAlert /> : <AlertTriangle />
              }
              title={error.title}
              body={error.body}
              action={
                error.kind === "generic" && error.onRetry ? (
                  <Button variant="outline" size="sm" onClick={error.onRetry}>
                    <RotateCw /> {t("common.tryAgain")}
                  </Button>
                ) : undefined
              }
            />
          </div>
        ) : error && isEmpty ? (
          <SectionErrorNotice error={error} />
        ) : (
          <>
            {/* A failed refresh over rows already on screen (or a draft)
                says so above them and keeps them: replacing them would lose
                what the user was reading or editing. */}
            {error ? <SectionErrorNotice error={error} spaced /> : null}
            <div className={ENV_LIST_CLASS}>
              {isEmpty ? null : <EnvListHead {...head} />}
              {children}
              {isEmpty ? (
                <EmptyCopy
                  title={empty.title}
                  body={empty.body}
                  action={empty.action}
                />
              ) : null}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

/**
 * The badge + explanation that names the blueprint manifest as the owner of a
 * value. Rendered wherever a manifest-owned row appears — read mode and the
 * draft — so the reason a row has no controls is always stated, never implied
 * by their absence.
 */
function ManifestManagedNote() {
  const { t } = useTranslations();
  return (
    <>
      <Badge variant="outline" className="gap-1">
        <FileCode2 aria-hidden="true" className="size-3" />
        {t("services.envManifestManaged")}
      </Badge>
      <p className="text-muted-foreground w-full text-sm max-md:text-base">
        {t("services.envManifestManagedBody")}
      </p>
    </>
  );
}

function SensitiveViewItem({
  name,
  value,
  loading,
  revealDisabled,
  revealReason,
  readOnlyNote = false,
  onToggle,
  onCopy,
}: {
  name: string;
  value: string | undefined;
  loading: boolean;
  revealDisabled: boolean;
  revealReason?: string;
  /** Name the blueprint manifest as this value's owner (w4/m120). */
  readOnlyNote?: boolean;
  onToggle: () => void;
  onCopy: () => Promise<void>;
}) {
  const { t } = useTranslations();
  const visible = value !== undefined;
  return (
    <div className={ENV_ROW_CLASS}>
      {readOnlyNote ? (
        <div
          className={cn(
            "flex min-w-0 flex-wrap items-center gap-2",
            ENV_ROW_NAME_CLASS,
          )}
        >
          <code className="min-w-0 text-sm font-medium wrap-anywhere">
            {name}
          </code>
          <ManifestManagedNote />
        </div>
      ) : (
        <code
          className={cn(
            "min-w-0 text-sm font-medium wrap-anywhere",
            ENV_ROW_NAME_CLASS,
          )}
        >
          {name}
        </code>
      )}
      {/* whitespace-pre-wrap, because a secret FILE is a file: a PEM key, a
          service-account JSON, a certificate chain. The default `normal`
          collapsed every newline, so a 20-line key revealed as one unreadable
          run-on line on the single screen where a user can check a secret they
          cannot see anywhere else (w4/108). The bytes were always correct —
          only the rendering was not. break-all stays for long unbroken base64
          runs, and the height is bounded so a 40-line PEM scrolls inside its
          row instead of pushing the page. Env VALUES share this component and
          have supported line breaks since w2/m95, so they gain it too. */}
      <div className={ENV_ROW_VALUE_CLASS}>
        <code
          className="block max-h-48 min-w-0 flex-1 overflow-auto whitespace-pre-wrap break-all text-sm"
          aria-label={visible ? value : t("services.environmentMaskedValue")}
        >
          {visible ? value : MASKED_VALUE}
        </code>
      </div>
      <div className={ENV_ROW_ACTIONS_CLASS}>
        <PermissionTooltip
          reason={!visible && revealDisabled ? revealReason : undefined}
        >
          <Button
            size="sm"
            variant="ghost"
            disabled={loading || (!visible && revealDisabled)}
            aria-label={t(
              visible ? "services.envHideOne" : "services.envRevealOne",
              {
                name,
              },
            )}
            onClick={onToggle}
          >
            {loading ? (
              <Loader2 className="animate-spin" />
            ) : visible ? (
              <EyeOff />
            ) : (
              <Eye />
            )}
            {/* Both labels occupy the same cell and the hidden one keeps its
                width, so toggling never moves the value field beside it. The
                button's name comes from aria-label. */}
            <span className="grid [&>*]:col-start-1 [&>*]:row-start-1">
              <span className={cn(visible && "invisible")}>
                {t("services.envReveal")}
              </span>
              <span className={cn(!visible && "invisible")}>
                {t("services.envHide")}
              </span>
            </span>
          </Button>
        </PermissionTooltip>
        <PermissionTooltip reason={revealDisabled ? revealReason : undefined}>
          <Button
            size="icon"
            variant="ghost"
            disabled={loading || revealDisabled}
            aria-label={t("services.envCopyOne", { name })}
            onClick={() => void onCopy()}
          >
            <Copy />
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
    <div
      className={cn(
        "flex min-w-0 items-center justify-between gap-3 py-4 first:pt-0 last:pb-0 @2xl/page:py-3",
        ENV_LIST_FULL_ROW_CLASS,
      )}
    >
      <code className="min-w-0 text-sm line-through opacity-60 wrap-anywhere">
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
  error?: DraftValidation["env"][string];
  disabled: boolean;
  permissionDescriptionID?: string;
  onChange: (update: Partial<EnvDraftRow>) => void;
}) {
  const { t } = useTranslations();
  const valueErrorId = useId();
  // A pristine new row's empty key is invalid but hasn't been gotten *wrong*
  // yet: the rule renders as neutral helper text until the user types or
  // leaves the field, as a secret file's name does (w6/057, w4/543).
  const [touched, setTouched] = useState(false);
  // The value rules (a NUL, an entry over the process limit, a PORT the
  // operator will not serve on) mark the value, not the key (w21/047).
  const valueRule =
    error === "nul" || error === "tooLarge" || error === "portInvalid"
      ? error
      : undefined;
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
  // A manifest-owned row has no key field, no value field and no delete — the
  // draft carries it only so the user can see it is there and why it can't be
  // changed here. `environmentDraftPatch` skips it too, so even a control
  // reintroduced by mistake could not produce a write.
  if (row.readOnly) {
    return (
      <div className="grid min-w-0 gap-2 py-4 first:pt-0 last:pb-0 @2xl/page:col-span-full @2xl/page:grid-cols-subgrid @2xl/page:items-start @2xl/page:py-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <code className="min-w-0 text-sm font-medium wrap-anywhere">
            {row.key}
          </code>
          <ManifestManagedNote />
        </div>
        <div className={ENV_ROW_VALUE_CLASS}>
          <code
            className="min-w-0 break-all text-sm"
            aria-label={t("services.environmentMaskedValue")}
          >
            {MASKED_VALUE}
          </code>
        </div>
      </div>
    );
  }
  // A saved row whose key is cleared was gotten wrong at once.
  const showKeyError =
    Boolean(error) &&
    !valueRule &&
    (touched || row.key !== "" || !isNewDraftRow(row));
  return (
    // Narrow: the key with its delete beside it, the value under both (two
    // lines, not three with a lone delete). Wide: the table's columns.
    <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-2 py-4 first:pt-0 last:pb-0 @2xl/page:col-span-full @2xl/page:grid-cols-subgrid @2xl/page:items-start @2xl/page:gap-y-0 @2xl/page:py-3">
      <div className="min-w-0 space-y-1">
        <Input
          value={row.key}
          disabled={disabled}
          aria-describedby={disabled ? permissionDescriptionID : undefined}
          onChange={(event) => onChange({ key: event.target.value })}
          onBlur={() => setTouched(true)}
          className="min-w-0 font-mono text-sm"
          // Key keeps the generic column name — its own value *is* the
          // variable identity; interpolating would be circular (m101/t013).
          aria-label={t("services.envColKey")}
          aria-invalid={showKeyError}
          placeholder={t("services.envKeyPlaceholder")}
        />
        {showKeyError ? (
          <p className="text-destructive text-xs" role="alert">
            {/* Two blank keys collide, but what's wrong is that they're blank. */}
            {error === "duplicate" && row.key !== ""
              ? t("services.environmentDuplicateKey")
              : error === "value"
                ? t("services.environmentValueRequired")
                : error === "limit"
                  ? t("services.environmentLimit")
                  : t("services.envInvalidKey")}
          </p>
        ) : isNewDraftRow(row) && row.key === "" ? (
          <p className="text-muted-foreground text-xs">
            {t("services.envInvalidKey")}
          </p>
        ) : null}
      </div>
      <div className="col-span-2 row-start-2 min-w-0 space-y-1 @2xl/page:col-span-1 @2xl/page:row-start-auto">
        <AutoTextarea
          value={row.value ?? ""}
          disabled={disabled}
          aria-describedby={
            valueRule
              ? valueErrorId
              : disabled
                ? permissionDescriptionID
                : undefined
          }
          aria-invalid={valueRule ? true : undefined}
          onChange={(event) =>
            onChange({
              value: event.target.value,
              valueChanged: true,
              generateValue: false,
            })
          }
          className="min-w-0 font-mono text-sm"
          aria-label={
            row.key.trim()
              ? t("services.envValueFor", { name: row.key.trim() })
              : t("services.envColValue")
          }
          placeholder={
            row.originalKey && !row.valueChanged
              ? t("services.environmentUnchangedMasked")
              : t("services.envValuePlaceholder")
          }
        />
        {valueRule ? (
          <p
            id={valueErrorId}
            className="text-destructive text-xs"
            role="alert"
          >
            {valueRule === "nul"
              ? t("services.environmentValueNul")
              : valueRule === "tooLarge"
                ? t("services.environmentValueTooLarge")
                : t("services.environmentPortInvalid")}
          </p>
        ) : null}
      </div>
      <Button
        size="icon"
        variant="ghost"
        className="col-start-2 row-start-1 @2xl/page:col-start-auto @2xl/page:row-start-auto"
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        aria-label={
          row.key.trim()
            ? t("services.envDeleteOne", { name: row.key.trim() })
            : t("services.envDelete")
        }
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
  error?: DraftValidation["files"][string];
  disabled: boolean;
  permissionDescriptionID?: string;
  onChange: (update: Partial<SecretFileDraftRow>) => void;
  onContent: () => void;
}) {
  const { t } = useTranslations();
  // A pristine new row's empty name is invalid but hasn't been gotten *wrong*
  // yet — the rule renders as neutral helper text until the user types or
  // leaves the field, and only then in error styling (w6/057).
  const [touched, setTouched] = useState(false);
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
  const showError = Boolean(error) && (touched || row.name !== "");
  return (
    // Narrow: the name with its delete beside it, then the contents control.
    // Wide: the table's columns, the contents control under "Contents".
    <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-2 py-4 first:pt-0 last:pb-0 @2xl/page:col-span-full @2xl/page:grid-cols-subgrid @2xl/page:items-start @2xl/page:gap-y-0 @2xl/page:py-3">
      <div className="min-w-0 space-y-1">
        <Input
          value={row.name}
          disabled={disabled}
          aria-describedby={disabled ? permissionDescriptionID : undefined}
          onChange={(event) => onChange({ name: event.target.value })}
          onBlur={() => setTouched(true)}
          className="min-w-0 font-mono text-sm"
          // File name keeps the generic column name, same decision as Key.
          aria-label={t("services.secretFileColName")}
          aria-invalid={showError}
          placeholder={t("services.secretFileNamePlaceholder")}
        />
        {showError ? (
          <p className="text-destructive text-xs" role="alert">
            {error === "duplicate"
              ? t("services.secretFileDuplicateName")
              : error === "content"
                ? t("services.secretFileContentRequired")
                : error === "limit"
                  ? t("services.secretFileLimit")
                  : t("services.secretFileInvalidName")}
          </p>
        ) : isNewDraftRow(row) ? (
          <p className="text-muted-foreground text-xs">
            {t("services.secretFileNameHint")}
          </p>
        ) : null}
      </div>
      <Button
        variant="outline"
        className="col-span-2 row-start-2 justify-self-start @2xl/page:col-span-1 @2xl/page:row-start-auto"
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        aria-label={
          row.name.trim()
            ? row.contentChanged
              ? t("services.secretFileEditContentOne", {
                  name: row.name.trim(),
                })
              : t("services.secretFileViewContentOne", {
                  name: row.name.trim(),
                })
            : undefined
        }
        onClick={onContent}
      >
        {row.contentChanged ? <Pencil /> : <Eye />}
        {row.contentChanged
          ? t("services.secretFileEditContent")
          : t("services.secretFileViewContent")}
      </Button>
      <Button
        size="icon"
        variant="ghost"
        className="col-start-2 row-start-1 @2xl/page:col-start-auto @2xl/page:row-start-auto"
        disabled={disabled}
        aria-describedby={disabled ? permissionDescriptionID : undefined}
        aria-label={
          row.name.trim()
            ? t("services.secretFileDeleteOne", { name: row.name.trim() })
            : t("services.secretFileDelete")
        }
        onClick={() => onChange({ deleted: true })}
      >
        <Trash2 className="text-destructive" />
      </Button>
    </div>
  );
}

/**
 * Render's add control: "Add variable" adds a blank row in one click, and the
 * adjacent menu holds the other ways in — a generated secret, a .env import.
 * The menu's root sits outside the reason tooltip: PermissionTooltip changes
 * its child's tree position when a reason comes and goes (e.g. a capability
 * refresh passing through "checking"), and a remounted root drops its open
 * state (w4/260). Only the trigger moves; the open menu stays, its items
 * disabled while the trigger is.
 */
function AddVariableMenu({
  disabled,
  reason,
  onAdd,
  onGenerate,
  onImport,
}: {
  disabled: boolean;
  reason?: string;
  onAdd: () => void;
  onGenerate: () => void;
  onImport: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex">
      <PermissionTooltip reason={reason}>
        <Button
          size="sm"
          variant="outline"
          className="rounded-r-none"
          disabled={disabled}
          onClick={onAdd}
        >
          <Plus /> {t("services.envAddVariable")}
        </Button>
      </PermissionTooltip>
      <DropdownMenu>
        <PermissionTooltip reason={reason}>
          <DropdownMenuTrigger asChild>
            <Button
              size="sm"
              variant="outline"
              className="rounded-l-none border-l-0 px-2"
              disabled={disabled}
              aria-label={t("services.envAddOptions")}
            >
              <ChevronDown />
            </Button>
          </DropdownMenuTrigger>
        </PermissionTooltip>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled={disabled} onSelect={onGenerate}>
            <Sparkles /> {t("services.envAddGenerated")}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={disabled} onSelect={onImport}>
            <FileUp /> {t("services.envImport")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

function EmptyCopy({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action?: React.ReactNode;
}) {
  return (
    <div className={cn("py-6 text-center", ENV_LIST_FULL_ROW_CLASS)}>
      <p className="font-medium">{title}</p>
      <p className="text-muted-foreground mt-1 text-sm text-pretty max-md:text-base">
        {body}
      </p>
      {action ? <div className="mt-3 flex justify-center">{action}</div> : null}
    </div>
  );
}

const SKIP_REASON_KEYS = {
  name: "services.secretFileSkippedName",
  size: "services.secretFileSkippedSize",
  encoding: "services.secretFileSkippedEncoding",
  duplicate: "services.secretFileSkippedDuplicate",
} as const;

/** Each file an upload left out, with its own reason (w4/628). */
function SkippedUploadsNotice({
  skipped,
}: {
  skipped: readonly SkippedUpload[];
}) {
  const { t } = useTranslations();
  return (
    <>
      <p>{t("services.secretFileSkippedTitle", { count: skipped.length })}</p>
      <ul className="mt-1 list-disc space-y-0.5 pl-5">
        {skipped.map((skip, index) => (
          <li key={`${skip.fileName}:${index}`}>
            <span className="font-mono break-all">{skip.fileName}</span>
            {": "}
            {t(SKIP_REASON_KEYS[skip.reason])}
          </li>
        ))}
      </ul>
    </>
  );
}
