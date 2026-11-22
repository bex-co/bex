import { useId, useState } from "react";
import { FileUp, Plus, Trash2, Sparkles } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { AutoTextarea } from "@/common/components/ui/auto-textarea";
import { CreateSection } from "@/common/components/create-form";
import { EnvImportDialog } from "@/features/services/components/env-import-dialog";
import {
  upsertDotenvEntries,
  type DotenvEntry,
} from "@/features/services/lib/dotenv-import";
import {
  duplicateNames,
  envValueRuleError,
  VALID_ENV_KEY,
} from "@/features/services/lib/environment-draft";
import { generateEnvValue } from "@/features/services/lib/generate-env-value";
import { removeRow, updateRow } from "@/features/services/lib/row-editor";
import type { EnvVarEntry } from "@/features/services/hooks/use-create-service";

const EMPTY_ROW: EnvVarEntry = { key: "", value: "" };

/**
 * Inline key-value editor for create-time env vars (Render parity, w5/m19):
 * Render's Environment Variables card, which opens on one empty KEY / VALUE
 * row and adds rows or a pasted `.env` below it. The empty row is drawn, not
 * stored, so an untouched card submits nothing.
 */
export function CreateEnvVarEditor({
  rows,
  onChange,
}: {
  rows: EnvVarEntry[];
  onChange: (rows: EnvVarEntry[]) => void;
}) {
  const { t } = useTranslations();

  const baseId = useId();
  const [importOpen, setImportOpen] = useState(false);
  const shown = rows.length > 0 ? rows : [EMPTY_ROW];
  const duplicates = duplicateNames(rows.map((row) => row.key));

  function addRow() {
    onChange([...shown, { key: "", value: "" }]);
  }

  function patchRow(i: number, patch: Partial<EnvVarEntry>) {
    onChange(updateRow(shown, i, patch));
  }

  function importEntries(entries: DotenvEntry[]) {
    onChange(
      upsertDotenvEntries(
        rows.filter((row) => row.key !== "" || row.value !== ""),
        entries,
        (row) => row.key,
        (row, entry) => ({ ...row, value: entry.value }),
        (entry) => ({ key: entry.key, value: entry.value }),
      ),
    );
  }

  return (
    <CreateSection
      title={t("services.createFieldEnvVarsTitle")}
      description={t("services.createEnvVarsDescription")}
    >
      {/* From @2xl the header and every row share one grid (rows are
          `contents`), so the headings sit over their columns whatever the
          buttons' widths in the active language. */}
      <div className="space-y-1.5 @2xl:grid @2xl:grid-cols-[1fr_1fr_auto_auto] @2xl:gap-x-2 @2xl:gap-y-1.5 @2xl:space-y-0">
        <span className="hidden px-0.5 text-xs text-muted-foreground @2xl:block">
          {t("services.createFieldEnvVarsKey")}
        </span>
        <span className="hidden px-0.5 text-xs text-muted-foreground @2xl:block">
          {t("services.createFieldEnvVarsValue")}
        </span>
        <span className="hidden @2xl:block" />
        <span className="hidden @2xl:block" />
        {shown.map((row, i) => {
          const keySyntaxInvalid =
            row.key !== "" && !VALID_ENV_KEY.test(row.key);
          const keyDuplicate = duplicates.has(row.key.trim());
          const keyInvalid = keySyntaxInvalid || keyDuplicate;
          // The value rules bex-api refuses at create (a bad PORT, an entry
          // over the process limit, a NUL) mark the value, as in the Edit
          // editor (w21/050).
          const valueRule =
            row.key === "" ? null : envValueRuleError(row.key, row.value);
          return (
            <div
              key={i}
              className="space-y-1 border-b pb-3 last:border-b-0 last:pb-0 @2xl:contents @2xl:space-y-0"
            >
              <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] gap-2 @2xl:contents">
                <Input
                  value={row.key}
                  onChange={(e) => patchRow(i, { key: e.target.value })}
                  placeholder={t("services.createFieldEnvVarsKeyPlaceholder")}
                  // Named by row so three rows don't read alike (w4/544).
                  aria-label={t("services.envRowKey", { n: String(i + 1) })}
                  aria-invalid={keyInvalid}
                  aria-describedby={
                    keyInvalid ? `${baseId}-${i}-error` : undefined
                  }
                  className="col-span-3 h-11 font-mono text-sm sm:h-9 @2xl:col-span-1"
                />
                <AutoTextarea
                  value={row.value}
                  onChange={(e) => patchRow(i, { value: e.target.value })}
                  placeholder={t("services.createFieldEnvVarsValuePlaceholder")}
                  aria-label={t("services.envRowValue", {
                    n: String(i + 1),
                  })}
                  aria-invalid={valueRule ? true : undefined}
                  aria-describedby={
                    valueRule ? `${baseId}-${i}-value-error` : undefined
                  }
                  className="min-h-11 py-2.5 font-mono text-sm sm:min-h-9 sm:py-1.5"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-11 min-w-11 sm:h-9 sm:min-w-9"
                  onClick={() => patchRow(i, { value: generateEnvValue() })}
                  aria-label={t("services.envRowGenerate", {
                    n: String(i + 1),
                  })}
                >
                  <Sparkles className="size-3.5" />
                  <span className="sr-only @md:not-sr-only">
                    {t("services.envGenerate")}
                  </span>
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-11 sm:size-9"
                  onClick={() => onChange(removeRow(shown, i))}
                  aria-label={t("services.envRowRemove", {
                    n: String(i + 1),
                  })}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
              {valueRule && (
                <p
                  id={`${baseId}-${i}-value-error`}
                  role="alert"
                  className="text-xs text-destructive @2xl:col-span-4"
                >
                  {valueRule === "nul"
                    ? t("services.environmentValueNul")
                    : valueRule === "tooLarge"
                      ? t("services.environmentValueTooLarge")
                      : t("services.environmentPortInvalid")}
                </p>
              )}
              {keyInvalid && (
                <p
                  id={`${baseId}-${i}-error`}
                  role="alert"
                  className="text-xs text-destructive @2xl:col-span-4"
                >
                  {keySyntaxInvalid
                    ? t("services.createFieldEnvVarsKeyError")
                    : t("services.environmentDuplicateKey")}
                </p>
              )}
            </div>
          );
        })}
      </div>
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="outline"
          className="h-11 sm:h-9"
          onClick={addRow}
        >
          <Plus className="size-3.5" />
          {t("services.createFieldEnvVarsAdd")}
        </Button>
        <Button
          type="button"
          variant="outline"
          className="h-11 sm:h-9"
          onClick={() => setImportOpen(true)}
        >
          <FileUp className="size-3.5" />
          {t("services.envImport")}
        </Button>
      </div>
      <EnvImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImport={importEntries}
      />
    </CreateSection>
  );
}
