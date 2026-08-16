import { Plus, Trash2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Textarea } from "@/common/components/ui/textarea";
import { isValidSecretFileName } from "@/features/services/lib/environment-draft";
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
        const invalid = row.name !== "" && !isValidSecretFileName(row.name);
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
                  aria-label={t("services.createFieldSecretFilesName")}
                  aria-invalid={invalid}
                  className="font-mono text-sm"
                />
                <Textarea
                  value={row.content}
                  onChange={(e) => patchRow(i, { content: e.target.value })}
                  placeholder={t(
                    "services.createFieldSecretFilesContentPlaceholder",
                  )}
                  aria-label={t("services.createFieldSecretFilesContent")}
                  className="min-h-20 font-mono text-sm"
                />
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={() => onChange(removeRow(rows, i))}
                aria-label={t("services.createFieldSecretFilesRemove")}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
            {invalid ? (
              <p className="text-xs text-destructive">
                {t("services.createFieldSecretFilesNameError")}
              </p>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
