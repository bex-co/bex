import { useId } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { CreateField } from "@/common/components/create-form";
import { Textarea } from "@/common/components/ui/textarea";
import {
  duplicateNames,
  isValidSecretFileName,
  secretFilesOverLimit,
} from "@/features/services/lib/environment-draft";
import { submittableSecretFiles } from "@/features/services/lib/create-service-input";
import { removeRow, updateRow } from "@/features/services/lib/row-editor";
import type { SecretFileEntry } from "@/features/services/hooks/use-create-service";

/** Create-time files mounted read-only at /etc/secrets from first boot. */
export function CreateSecretFileEditor({
  rows,
  onChange,
}: {
  rows: SecretFileEntry[];
  onChange: (rows: SecretFileEntry[]) => void;
}) {
  const { t } = useTranslations();
  const baseId = useId();
  const duplicates = duplicateNames(rows.map((row) => row.name));
  // The Environment editor's quota, checked before Submit (w4/658).
  const overLimit = secretFilesOverLimit(submittableSecretFiles(rows));
  const limitId = `${baseId}-limit`;

  function addRow() {
    onChange([...rows, { name: "", content: "" }]);
  }

  function patchRow(i: number, patch: Partial<SecretFileEntry>) {
    onChange(updateRow(rows, i, patch));
  }

  // A create row, as Render's Advanced lays out Secret Files: what they are
  // on the left, the files and the add button on the right.
  return (
    <CreateField
      label={t("services.createFieldSecretFilesTitle")}
      description={t("services.createFieldSecretFilesHint")}
    >
      {rows.map((row, i) => {
        const syntaxInvalid =
          row.name !== "" && !isValidSecretFileName(row.name);
        const duplicate = duplicates.has(row.name.trim());
        const invalid = syntaxInvalid || duplicate;
        return (
          <div key={i} className="space-y-1 rounded-md border p-3">
            <div className="flex items-start gap-2">
              <div className="flex-1 space-y-2">
                <Input
                  value={row.name}
                  onChange={(e) => patchRow(i, { name: e.target.value })}
                  placeholder={t(
                    "services.createFieldSecretFilesNamePlaceholder",
                  )}
                  // Named by row so two files don't read alike (w4/544).
                  aria-label={t("services.fileRowName", { n: String(i + 1) })}
                  aria-invalid={invalid}
                  aria-describedby={
                    invalid ? `${baseId}-${i}-error` : undefined
                  }
                  className="h-11 font-mono text-sm sm:h-9"
                />
                <Textarea
                  value={row.content}
                  onChange={(e) => patchRow(i, { content: e.target.value })}
                  placeholder={t(
                    "services.createFieldSecretFilesContentPlaceholder",
                  )}
                  aria-label={t("services.fileRowContent", {
                    n: String(i + 1),
                  })}
                  aria-invalid={overLimit && row.name !== ""}
                  aria-describedby={
                    overLimit && row.name !== "" ? limitId : undefined
                  }
                  className="min-h-20 font-mono text-sm"
                />
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-11 sm:size-9"
                onClick={() => onChange(removeRow(rows, i))}
                aria-label={t("services.fileRowRemove", { n: String(i + 1) })}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
            {invalid ? (
              <p
                id={`${baseId}-${i}-error`}
                role="alert"
                className="text-xs text-destructive"
              >
                {syntaxInvalid
                  ? t("services.createFieldSecretFilesNameError")
                  : t("services.secretFileDuplicateName")}
              </p>
            ) : null}
          </div>
        );
      })}
      {overLimit ? (
        <p id={limitId} role="alert" className="text-xs text-destructive">
          {t("services.secretFileLimit")}
        </p>
      ) : null}
      <Button
        type="button"
        variant="outline"
        className="h-11 sm:h-9"
        onClick={addRow}
      >
        <Plus className="size-3.5" />
        {t("services.createFieldSecretFilesAdd")}
      </Button>
    </CreateField>
  );
}
