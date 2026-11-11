import { useId } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Textarea } from "@/common/components/ui/textarea";
import {
  duplicateNames,
  isValidSecretFileName,
} from "@/features/services/lib/environment-draft";
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

  function addRow() {
    onChange([...rows, { name: "", content: "" }]);
  }

  function patchRow(i: number, patch: Partial<SecretFileEntry>) {
    onChange(updateRow(rows, i, patch));
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div>
          <Label>{t("services.createFieldSecretFilesTitle")}</Label>
          <p className="text-sm text-muted-foreground">
            {t("services.createFieldSecretFilesHint")}
          </p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="size-3.5" />
          {t("services.createFieldSecretFilesAdd")}
        </Button>
      </div>
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
                  className="font-mono text-sm"
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
                  className="min-h-20 font-mono text-sm"
                />
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
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
    </div>
  );
}
