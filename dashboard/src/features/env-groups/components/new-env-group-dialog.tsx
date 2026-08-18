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
import { EnvImportDialog } from "@/features/services/components/env-import-dialog";
import {
  upsertDotenvEntries,
  type DotenvEntry,
} from "@/features/services/lib/dotenv-import";

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
}

/** One-step workspace create: name, initial contents, and links share one mutation. */
export function NewEnvGroupDialog({
  onCreated,
  refetch,
  services = [],
  servicesLoading = false,
  open: openProp,
  onOpenChange: onOpenChangeProp,
  initialServiceIds = [],
}: NewEnvGroupDialogProps) {
  const { t } = useTranslations();
  const { createGroup, busy } = useEnvGroupMutations(refetch);
  const controlled = openProp !== undefined;
  const nextRowId = useRef(0);
  const [openState, setOpenState] = useState(false);
  const [name, setName] = useState("");
  const [envVars, setEnvVars] = useState<EnvVarRow[]>([]);
  const [secretFiles, setSecretFiles] = useState<SecretFileRow[]>([]);
  const [serviceIds, setServiceIds] = useState<string[]>(initialServiceIds);
  const [invalid, setInvalid] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const open = controlled ? openProp : openState;
  const contentsValid =
    envVars.every((variable) => isValidEnvVarKey(variable.key)) &&
    secretFiles.every((file) => isValidSecretFileName(file.name));
  const canSubmit = isValidEnvGroupName(name) && !busy;

  function reset() {
    setName("");
    setEnvVars([]);
    setSecretFiles([]);
    setServiceIds(initialServiceIds);
    setInvalid(false);
    setImportOpen(false);
  }

  function handleOpenChange(next: boolean) {
    if (controlled) onOpenChangeProp?.(next);
    else setOpenState(next);
    if (!next) reset();
    else setServiceIds(initialServiceIds);
  }

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
    if (!isValidEnvGroupName(name) || !contentsValid) {
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
      serviceIds,
    });
    if (!id) return;
    handleOpenChange(false);
    onCreated(id);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
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
            autoComplete="off"
          />
          {invalid && !isValidEnvGroupName(name) ? (
            <p className="text-destructive text-sm">
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
                  aria-invalid={invalid && !isValidEnvVarKey(variable.key)}
                  placeholder={t("envGroups.varKeyPlaceholder")}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor={`env-group-var-value-${variable.id}`}>
                  {t("envGroups.varValueLabel")}
                </Label>
                <Input
                  id={`env-group-var-value-${variable.id}`}
                  type="password"
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
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={addEnvVar}
            >
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
            <div
              key={file.id}
              className="grid gap-2 sm:grid-cols-[1fr_2fr_auto]"
            >
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
            <h3 className="font-medium">
              {t("envGroups.createServicesTitle")}
            </h3>
            <p className="text-sm text-muted-foreground">
              {t("envGroups.createServicesDescription")}
            </p>
          </div>
          {servicesLoading ? (
            <p className="text-sm text-muted-foreground">
              {t("envGroups.servicesLoading")}
            </p>
          ) : services.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("envGroups.noServicesToLink")}
            </p>
          ) : (
            <div className="grid gap-2 sm:grid-cols-2">
              {services.map((service) => {
                const checkboxId = `env-group-service-${service.id}`;
                return (
                  <div
                    key={service.id}
                    className="flex items-center gap-2 rounded-md border p-3"
                  >
                    <Checkbox
                      id={checkboxId}
                      checked={serviceIds.includes(service.id)}
                      disabled={busy}
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
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={busy}
          >
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
      </DialogContent>
    </Dialog>
  );
}
