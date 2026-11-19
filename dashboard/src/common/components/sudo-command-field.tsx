import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import {
  aroundPlaceholder,
  PLACEHOLDER_SENTINEL,
} from "@/common/lib/i18n-placeholder";

export interface SudoCommandFieldProps {
  /** DOM id for the input (unique per dialog). */
  id: string;
  /** i18n key of a "Type {phrase} below to confirm." template. */
  promptKey: keyof typeof en;
  /** The exact sudo phrase the user must type, e.g. "sudo delete web service api". */
  phrase: string;
  value: string;
  onValueChange: (value: string) => void;
  inputClassName?: string;
}

/**
 * Render's sudo type-to-confirm block (live capture:
 * docs/render-artifacts/workspace-lifecycle.md): the "Type <phrase> below to
 * confirm." instruction is ordinary body copy with the exact phrase in bold —
 * never the input's label — and the input itself is labeled "Sudo Command".
 * Every destructive typed-confirmation gate shares this component so the
 * dialogs stay identical across services, Postgres, Key Value, and workspaces.
 */
export function SudoCommandField({
  id,
  promptKey,
  phrase,
  value,
  onValueChange,
  inputClassName,
}: SudoCommandFieldProps) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2">
      <p className="text-sm text-muted-foreground">
        <SudoPrompt promptKey={promptKey} phrase={phrase} />
      </p>
      <Label htmlFor={id}>{t("common.sudoCommandLabel")}</Label>
      <Input
        id={id}
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
        autoComplete="off"
        placeholder={phrase}
        className={inputClassName}
      />
    </div>
  );
}

/**
 * The "Type <phrase> below to confirm." copy with the phrase in bold: the
 * field's instruction, and the text its loading frames reserve.
 */
export function SudoPrompt({
  promptKey,
  phrase,
}: {
  promptKey: keyof typeof en;
  phrase: string;
}) {
  const { t } = useTranslations();
  const [before, after] = aroundPlaceholder(
    t(promptKey, { phrase: PLACEHOLDER_SENTINEL }),
  );
  return (
    <>
      {before}
      <strong className="font-semibold text-foreground">{phrase}</strong>
      {after}
    </>
  );
}
