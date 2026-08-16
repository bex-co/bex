import { Plus, Trash2, Sparkles } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { VALID_ENV_KEY } from "@/features/services/lib/environment-draft";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";
import { removeRow, updateRow } from "@/features/services/lib/row-editor";
import type { EnvVarEntry } from "@/features/services/hooks/use-create-service";

/** Inline key-value editor for create-time env vars (Render parity, w5/m19). */
export function CreateEnvVarEditor({
  rows,
  onChange,
}: {
  rows: EnvVarEntry[];
  onChange: (rows: EnvVarEntry[]) => void;
}) {
  const { t } = useTranslations();

  function addRow() {
    onChange([...rows, { key: "", value: "" }]);
  }

  function patchRow(i: number, patch: Partial<EnvVarEntry>) {
    onChange(updateRow(rows, i, patch));
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{t("services.createFieldEnvVarsTitle")}</Label>
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="size-3.5" />
          {t("services.createFieldEnvVarsAdd")}
        </Button>
      </div>
      {rows.length > 0 && (
        <div className="space-y-1.5">
          <div className="grid grid-cols-[1fr_1fr_auto_auto] gap-2 px-0.5">
            <span className="text-xs text-muted-foreground">
              {t("services.createFieldEnvVarsKey")}
            </span>
            <span className="text-xs text-muted-foreground">
              {t("services.createFieldEnvVarsValue")}
            </span>
            <span />
            <span />
          </div>
          {rows.map((row, i) => {
            const keyInvalid = row.key !== "" && !VALID_ENV_KEY.test(row.key);
            return (
              <div key={i} className="space-y-1">
                <div className="grid grid-cols-[1fr_1fr_auto_auto] gap-2">
                  <Input
                    value={row.key}
                    onChange={(e) => patchRow(i, { key: e.target.value })}
                    placeholder={t("services.createFieldEnvVarsKeyPlaceholder")}
                    aria-label={t("services.createFieldEnvVarsKey")}
                    aria-invalid={keyInvalid}
                    className="font-mono text-sm"
                  />
                  <Input
                    value={row.value}
                    onChange={(e) => patchRow(i, { value: e.target.value })}
                    placeholder={t(
                      "services.createFieldEnvVarsValuePlaceholder",
                    )}
                    aria-label={t("services.createFieldEnvVarsValue")}
                    className="font-mono text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => patchRow(i, { value: generateEnvValue() })}
                  >
                    <Sparkles className="size-3.5" />
                    {t("services.envGenerate")}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => onChange(removeRow(rows, i))}
                    aria-label={t("services.createFieldEnvVarsRemove")}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
                {keyInvalid && (
                  <p className="text-xs text-destructive">
                    {t("services.createFieldEnvVarsKeyError")}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
