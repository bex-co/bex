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
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useSecretFileNames,
  useRevealSecretFile,
  useSecretFileMutations,
  classifySecretFileError,
} from "@/features/services/hooks/use-secret-files";
import { SecretFileRow } from "@/features/services/components/secret-file-row";
import { CenteredState } from "@/features/services/components/centered-state";

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
  const initialLoading = loading && names.length === 0 && !error;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.secretFilesTitle")}</CardTitle>
        <CardDescription>
          {t("services.secretFilesDescription")}
        </CardDescription>
        <CardAction>
          <AddFileButton setFile={setFile} disabled={gated || busy} />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <TableSkeleton />
        ) : names.length === 0 ? (
          <SecretFilesEmptyState />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-1/3">
                  {t("services.secretFileColName")}
                </TableHead>
                <TableHead>{t("services.secretFileColContent")}</TableHead>
                <TableHead className="sr-only text-right">actions</TableHead>
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

function SecretFilesEmptyState() {
  const { t } = useTranslations();
  return (
    <CenteredState
      icon={<FileLock2 />}
      title={t("services.secretFilesEmptyTitle")}
      body={t("services.secretFilesEmptyBody")}
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
      title: t("services.secretFilesUnavailableTitle"),
      body: t("services.secretFilesUnavailableBody"),
    },
    forbidden: {
      icon: <ShieldAlert />,
      title: t("services.secretFilesForbiddenTitle"),
      body: t("services.secretFilesForbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("services.secretFilesErrorTitle"),
      body: t("services.secretFilesErrorBody"),
    },
  }[kind];
  return <CenteredState icon={copy.icon} title={copy.title} body={copy.body} />;
}
