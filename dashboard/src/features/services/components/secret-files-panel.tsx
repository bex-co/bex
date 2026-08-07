import { useState } from "react";
import { Plus, FileLock2, ShieldAlert, AlertTriangle } from "lucide-react";
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
import { Textarea } from "@/common/components/ui/textarea";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useSecretFileNames,
  useRevealSecretFile,
  useSecretFileMutations,
  classifySecretFileError,
} from "@/features/services/hooks/use-secret-files";
import { SecretFileRow } from "@/features/services/components/secret-file-row";
import type { SecretFileName } from "@/features/services/types";
import type { SensitiveEditorErrorKind } from "./env-vars-panel";

// A secret-file name: a relative path segment bex-api (and Kubernetes' Secret-key
// validation) accepts — reject bad names client-side rather than round-tripping to
// a 400. Kept in sync with backend/internal/secrets/files.go validFileName, which
// also rejects "." and ".." explicitly.
const VALID_FILE_NAME = /^[-._a-zA-Z0-9]+$/;

function isValidFileName(name: string): boolean {
  return VALID_FILE_NAME.test(name) && name !== "." && name !== "..";
}

/**
 * The service Environment tab's Secret Files section (Render dashboard shape):
 * lists a service's secret-file names, reveals a file's content on demand, and
 * adds/updates/deletes files — all over bex-api's secret-files GraphQL. Content is
 * fetched per file, never in the list.
 */
export function SecretFilesPanel({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { names, loading, error, refetch } = useSecretFileNames(serviceId);
  const reveal = useRevealSecretFile(serviceId);
  const { setFile, deleteFile, busy } = useSecretFileMutations(
    serviceId,
    refetch,
  );

  const errorKind = classifySecretFileError(error);
  return (
    <SecretFilesEditor
      names={names}
      loading={loading}
      errorKind={errorKind}
      reveal={reveal}
      setFile={setFile}
      deleteFile={deleteFile}
      busy={busy}
      copy={{
        title: t("services.secretFilesTitle"),
        description: t("services.secretFilesDescription"),
        emptyTitle: t("services.secretFilesEmptyTitle"),
        emptyBody: t("services.secretFilesEmptyBody"),
        unavailableTitle: t("services.secretFilesUnavailableTitle"),
        unavailableBody: t("services.secretFilesUnavailableBody"),
        forbiddenTitle: t("services.secretFilesForbiddenTitle"),
        forbiddenBody: t("services.secretFilesForbiddenBody"),
        errorTitle: t("services.secretFilesErrorTitle"),
        errorBody: t("services.secretFilesErrorBody"),
        deleteConfirmBody: t("services.secretFileDeleteConfirmBody"),
      }}
    />
  );
}

export interface SecretFilesEditorCopy {
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

/** Shared names-only secret-file editor used by services and env groups. */
export function SecretFilesEditor({
  names,
  loading,
  errorKind,
  reveal,
  setFile,
  deleteFile,
  busy,
  copy,
}: {
  names: SecretFileName[];
  loading: boolean;
  errorKind: SensitiveEditorErrorKind | null;
  reveal: (name: string) => Promise<string>;
  setFile: (name: string, content: string) => Promise<boolean>;
  deleteFile: (name: string) => Promise<boolean>;
  busy: boolean;
  copy: SecretFilesEditorCopy;
}) {
  const { t } = useTranslations();
  const initialLoading = loading && names.length === 0 && !errorKind;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{copy.title}</CardTitle>
        <CardDescription>{copy.description}</CardDescription>
        <CardAction>
          <AddFileButton setFile={setFile} disabled={gated || busy} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} copy={copy} />
        ) : initialLoading ? (
          <PanelTableSkeleton rows={3} />
        ) : names.length === 0 ? (
          <PanelCenteredState
            icon={<FileLock2 />}
            title={copy.emptyTitle}
            body={copy.emptyBody}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-1/3">
                  {t("services.secretFileColName")}
                </TableHead>
                <TableHead>{t("services.secretFileColContent")}</TableHead>
                <TableHead className="sr-only text-right">
                  {t("services.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {names.map((entry) => (
                <SecretFileRow
                  key={entry.id}
                  entry={entry}
                  reveal={reveal}
                  onSave={setFile}
                  onDelete={deleteFile}
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

/** The "Add secret file" affordance: a button that opens an inline name+content form. */
function AddFileButton({
  setFile,
  disabled,
}: {
  setFile: (name: string, content: string) => Promise<boolean>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [content, setContent] = useState("");
  const [invalid, setInvalid] = useState(false);
  const [saving, setSaving] = useState(false);

  function reset() {
    setName("");
    setContent("");
    setInvalid(false);
    setOpen(false);
  }

  async function submit() {
    if (!isValidFileName(name.trim())) {
      setInvalid(true);
      return;
    }
    setSaving(true);
    const ok = await setFile(name.trim(), content);
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
        <Plus /> {t("services.secretFileAdd")}
      </Button>
    );
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-start gap-2">
        <Input
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            setInvalid(false);
          }}
          placeholder={t("services.secretFileNamePlaceholder")}
          aria-label={t("services.secretFileColName")}
          aria-invalid={invalid}
          className="w-40 font-mono text-sm"
          autoFocus
        />
        <Textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={t("services.secretFileContentPlaceholder")}
          aria-label={t("services.secretFileColContent")}
          className="w-56 font-mono text-sm"
          onKeyDown={(e) => {
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
          {t("services.secretFileInvalidName")}
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
  copy: SecretFilesEditorCopy;
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
