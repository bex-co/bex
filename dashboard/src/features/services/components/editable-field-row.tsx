import { useEffect, useRef, useState, type ReactNode } from "react";
import { Loader2, Pencil } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";

export interface EditableFieldSelectOption {
  value: string;
  label: string;
}

/** Confirm dialog shown before {@link EditableFieldRow} persists a change —
 *  used by rebuild-affecting fields (Branch, Root Directory, commands). */
export interface EditableFieldConfirm {
  /** Dialog title, given the value about to be saved (its empty stand-in when blank). */
  title: (value: string) => string;
  body: ReactNode;
  /** Stand-in used in the title when the draft is empty (e.g. "the repository root"). */
  emptyValue?: string;
}

export interface EditableFieldRowProps {
  /** Field label; also the input's default accessible name. */
  label: string;
  /** Help text under the label. */
  hint?: ReactNode;
  /** Renders an "Optional" badge beside the label. */
  optional?: boolean;
  /** Current persisted value; the input resets here and dirty-compares against it. */
  value: string;
  /** aria-label for the pencil Edit button (required — a11y + tests key off it).
   *  The input/select takes its accessible name from `label`. */
  editLabel: string;
  placeholder?: string;
  /** Monospace input text for paths / commands / schedules. */
  mono?: boolean;
  busy?: boolean;
  /** When true, the field is not editable: the input stays disabled and the
   *  pencil is hidden (e.g. a paid-plan-only field on the free plan). */
  disabled?: boolean;
  type?: "text" | "number";
  min?: number;
  max?: number;
  step?: number;
  /** Present ⇒ render a disabled Select instead of an Input (the select variant). */
  options?: readonly EditableFieldSelectOption[];
  /** Display-only prefix shown before the value (e.g. "app/ $"); never saved. */
  valuePrefix?: string;
  /**
   * Trim the draft before dirty-comparing and saving (default true). Set false
   * for fields where surrounding whitespace is meaningful or normalized in onSave.
   */
  trim?: boolean;
  /** Whether the change counts as savable; default `normalized !== value`.
   *  Receives the normalized draft (trimmed unless `trim` is false) — the same
   *  value `onSave` gets. */
  dirty?: (normalized: string) => boolean;
  /** Return an error to block save and show it inline; null ⇒ valid. Receives
   *  the raw draft (before normalization). */
  validate?: (draft: string) => string | null;
  /** Confirm dialog shown before onSave for rebuild-affecting fields. */
  confirm?: EditableFieldConfirm;
  /** Persist the (normalized) draft; return true on success to leave edit mode. */
  onSave: (value: string) => Promise<boolean>;
}

/**
 * Render's settings edit-in-place row (w5/m50): the field's current value always
 * lives in a real, visibly disabled input (or select). A pencil Edit button
 * enables and focuses that same control and swaps the pencil for Cancel + "Save
 * changes", Save staying disabled until the draft actually differs. Cancel (and
 * Escape) restores the value and the disabled state. Rebuild-affecting fields
 * pass a `confirm` dialog that runs — and can be dismissed — before onSave fires.
 *
 * One component, two variants: pass `options` for the select variant (consumed
 * by the Notifications row here and the Auto-Deploy row in w5/m53), otherwise a
 * text/number input.
 */
export function EditableFieldRow({
  label,
  hint,
  optional = false,
  value,
  editLabel,
  placeholder,
  mono = false,
  busy = false,
  disabled = false,
  type = "text",
  min,
  max,
  step,
  options,
  valuePrefix,
  trim = true,
  dirty,
  validate,
  confirm,
  onSave,
}: EditableFieldRowProps) {
  const { t } = useTranslations();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [confirming, setConfirming] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const editable = !disabled;
  // The disabled control always shows the persisted value; the draft only
  // exists while editing (seeded on Edit), so an external value update never
  // clobbers an in-progress edit.
  const shown = editing ? draft : value;

  // Focus (and select, for inputs) the control once editing enables it.
  useEffect(() => {
    if (!editing) return;
    if (options) triggerRef.current?.focus();
    else {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing, options]);

  const normalized = trim ? draft.trim() : draft;
  const validationError = editing ? (validate?.(draft) ?? null) : null;
  const isDirty = dirty ? dirty(normalized) : normalized !== value;
  const canSave = isDirty && validationError === null;

  function startEdit() {
    setDraft(value);
    setEditing(true);
  }

  function cancel() {
    setDraft(value);
    setEditing(false);
  }

  async function persist() {
    setConfirming(false);
    const ok = await onSave(normalized);
    if (ok) setEditing(false);
  }

  function requestSave() {
    if (!canSave) return;
    if (confirm) setConfirming(true);
    else void persist();
  }

  return (
    <div className="space-y-2">
      <div>
        <div className="flex items-center gap-2 text-sm font-medium">
          {label}
          {optional && (
            <Badge variant="outline" className="text-xs font-normal">
              {t("services.editRowOptional")}
            </Badge>
          )}
        </div>
        {hint != null && (
          <div className="text-muted-foreground mt-1 text-sm">{hint}</div>
        )}
      </div>

      <div className="flex items-center gap-2">
        {/* The value prefix (e.g. "app/ $") only makes sense before a text
            input, never a select. */}
        {!options && valuePrefix && (
          <code className="text-muted-foreground shrink-0 font-mono text-sm">
            {valuePrefix}
          </code>
        )}
        {options ? (
          <Select
            value={shown}
            disabled={disabled || !editing || busy}
            onValueChange={setDraft}
          >
            <SelectTrigger
              ref={triggerRef}
              size="sm"
              aria-label={label}
              className="flex-1"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {options.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <Input
            ref={inputRef}
            type={type}
            min={min}
            max={max}
            step={step}
            value={shown}
            disabled={disabled || !editing || busy}
            aria-label={label}
            aria-invalid={validationError !== null || undefined}
            placeholder={placeholder}
            autoComplete="off"
            className={cn("flex-1", mono && "font-mono text-sm")}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                requestSave();
              }
              if (event.key === "Escape") cancel();
            }}
          />
        )}

        {editing ? (
          <>
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={cancel}
            >
              {t("services.editRowCancel")}
            </Button>
            <Button
              size="sm"
              disabled={busy || !canSave}
              onClick={requestSave}
            >
              {busy && <Loader2 className="animate-spin" />}
              {t("services.editRowSave")}
            </Button>
          </>
        ) : (
          editable && (
            <Button
              size="icon"
              variant="ghost"
              aria-label={editLabel}
              onClick={startEdit}
            >
              <Pencil />
            </Button>
          )
        )}
      </div>

      {validationError !== null && (
        <p className="text-destructive text-sm">{validationError}</p>
      )}

      {confirm && (
        <AlertDialog
          open={confirming}
          onOpenChange={(open) => setConfirming(open)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {confirm.title(normalized || confirm.emptyValue || "")}
              </AlertDialogTitle>
              <AlertDialogDescription>{confirm.body}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>
                {t("services.editRowCancel")}
              </AlertDialogCancel>
              <AlertDialogAction onClick={() => void persist()}>
                {t("services.editRowSave")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      )}
    </div>
  );
}
