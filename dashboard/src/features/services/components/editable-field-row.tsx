import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { ChevronDown, Loader2, Pencil } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Combobox, type ComboboxOption } from "@/common/components/ui/combobox";
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
  /** Why the field is disabled — shown as a muted line beneath the row so a
   *  member whose role can't edit it learns why instead of hitting a 403 on
   *  save (w9/m84). Only rendered when `disabled` is also true. */
  disabledReason?: ReactNode;
  type?: "text" | "number";
  min?: number;
  max?: number;
  step?: number;
  /** Present ⇒ render a disabled Select instead of an Input (the select variant). */
  options?: readonly EditableFieldSelectOption[];
  /**
   * Present ⇒ render a searchable Combobox (typeahead over these options, with
   * free-text fallback) instead of an Input — the combobox variant used by the
   * Branch row (w5/m54). An empty list degrades to plain free-text entry.
   */
  comboboxOptions?: readonly ComboboxOption[];
  /** Display-only prefix shown before the value (e.g. "app/ $"); never saved. */
  valuePrefix?: string;
  /**
   * Maps the draft to the value that is dirty-compared and saved (default:
   * trim). Pass the identity, or a field's own policy, where surrounding
   * whitespace is meaningful or normalized in onSave.
   */
  normalize?: (draft: string) => string;
  /** Whether the change counts as savable; default `normalized !== value`.
   *  Receives the normalized draft — the same value `onSave` gets. */
  dirty?: (normalized: string) => boolean;
  /** Return an error to block save and show it inline; null ⇒ valid. Receives
   *  the raw draft (before normalization). */
  validate?: (draft: string) => string | null;
  /** Optional human-readable description of the shown value (the draft while
   *  editing, else the persisted value), rendered as a muted line beneath the
   *  row — e.g. a cron schedule's "Every 5 minutes". Return null for no line. */
  describe?: (value: string) => ReactNode | null;
  /** Confirm dialog shown before onSave for rebuild-affecting fields. */
  confirm?: EditableFieldConfirm;
  /** Persist the (normalized) draft; return true on success to leave edit mode. */
  onSave: (value: string) => Promise<boolean>;
}

const trimDraft = (draft: string) => draft.trim();

/**
 * Render's settings edit-in-place row (w5/m50): the field's current value always
 * lives in a real, visibly disabled input (or select). A pencil Edit button
 * enables and focuses that same control and swaps the pencil for Cancel + "Save
 * changes", Save staying disabled until the draft actually differs. Cancel (and
 * Escape) restores the value and the disabled state, and returns keyboard
 * focus to the pencil (w4/293). Rebuild-affecting fields
 * pass a `confirm` dialog that runs — and can be dismissed — before onSave fires.
 *
 * One component, three variants: pass `options` for the select variant (the
 * Notifications and Auto-Deploy rows), `comboboxOptions` for the searchable
 * combobox variant (the Branch row, w5/m54), otherwise a text/number input.
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
  disabledReason,
  type = "text",
  min,
  max,
  step,
  options,
  comboboxOptions,
  valuePrefix,
  normalize = trimDraft,
  dirty,
  validate,
  describe,
  confirm,
  onSave,
}: EditableFieldRowProps) {
  const { t } = useTranslations();
  const errorId = useId();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [confirming, setConfirming] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const editButtonRef = useRef<HTMLButtonElement>(null);
  // Set only by an explicit Cancel/Escape so the post-render effect returns
  // focus to the restored pencil — never on mount, a server refresh, or a
  // save (w4/293).
  const restoreFocusRef = useRef(false);
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

  // Cancel disables the focused control and unmounts Cancel/Save, which would
  // drop keyboard focus to <body>; hand it to this row's restored pencil once
  // it is mounted. A disabled row renders no pencil, so focus stays put.
  useEffect(() => {
    if (editing || !restoreFocusRef.current) return;
    restoreFocusRef.current = false;
    editButtonRef.current?.focus();
  }, [editing]);

  const normalized = normalize(draft);
  const validationError = editing ? (validate?.(draft) ?? null) : null;
  const description =
    validationError === null ? (describe?.(shown) ?? null) : null;
  const isDirty = dirty ? dirty(normalized) : normalized !== value;
  const canSave = isDirty && validationError === null;

  function startEdit() {
    setDraft(value);
    setEditing(true);
  }

  function cancel() {
    restoreFocusRef.current = true;
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

      <div
        className={cn(
          "flex items-center gap-2",
          // A long select label must not push the actions off a narrow
          // screen: the select wraps its label, and while editing it takes
          // the whole first line so Cancel/Save wrap beneath it (w4/251).
          options && "flex-wrap sm:flex-nowrap",
        )}
      >
        {/* The value prefix (e.g. "app/ $") only makes sense before a text
            input, never a select. */}
        {!options && valuePrefix && (
          <code className="text-muted-foreground shrink-0 font-mono text-sm">
            {valuePrefix}
          </code>
        )}
        {comboboxOptions ? (
          <div className="flex-1">
            <Combobox
              options={[...comboboxOptions]}
              value={shown}
              onValueChange={setDraft}
              disabled={disabled || !editing || busy}
              allowCustom
              ariaLabel={label}
              placeholder={placeholder}
              className={mono ? "font-mono text-sm" : undefined}
            />
          </div>
        ) : options ? (
          <Select
            value={shown}
            disabled={disabled || !editing || busy}
            onValueChange={setDraft}
          >
            <SelectTrigger
              ref={triggerRef}
              size="sm"
              aria-label={label}
              className={cn(
                "min-h-8 min-w-0 flex-1 text-left whitespace-normal data-[size=sm]:h-auto *:data-[slot=select-value]:line-clamp-none",
                editing && "basis-full sm:basis-0",
              )}
              // A closed trigger cancels on Escape like the Input; an open
              // popup consumes its own Escape and refocuses here (w4/250).
              onKeyDown={(event) => {
                if (event.key === "Escape" && editing && !busy) {
                  event.preventDefault();
                  cancel();
                }
              }}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="max-w-(--radix-select-content-available-width)">
              {options.map((option) => (
                <SelectItem
                  key={option.value}
                  value={option.value}
                  className="whitespace-normal"
                >
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
            aria-describedby={validationError !== null ? errorId : undefined}
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
            <Button variant="ghost" size="sm" disabled={busy} onClick={cancel}>
              {t("services.editRowCancel")}
            </Button>
            <Button size="sm" disabled={busy || !canSave} onClick={requestSave}>
              {busy && <Loader2 className="animate-spin" />}
              {t("services.editRowSave")}
            </Button>
          </>
        ) : (
          editable && (
            <Button
              ref={editButtonRef}
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

      {description != null && (
        <p className="text-muted-foreground text-sm">{description}</p>
      )}

      {validationError !== null && (
        <p id={errorId} className="text-destructive text-sm">
          {validationError}
        </p>
      )}

      {disabled && disabledReason != null && (
        <p className="text-muted-foreground text-sm">{disabledReason}</p>
      )}

      {confirm && (
        <ConfirmDialog
          open={confirming}
          onOpenChange={(open) => setConfirming(open)}
          title={confirm.title(normalized || confirm.emptyValue || "")}
          description={confirm.body}
          cancelLabel={t("services.editRowCancel")}
          confirmLabel={t("services.editRowSave")}
          // Saving an edit is the primary action, not a destructive one.
          destructive={false}
          // The title quotes the full new value, and a valid command may run
          // to thousands of characters: cap the dialog to the viewport and
          // scroll inside it so Save/Cancel stay reachable on mobile (w4/219).
          contentClassName="max-h-[85dvh] overflow-y-auto [overflow-wrap:anywhere]"
          onConfirm={() => void persist()}
        />
      )}
    </div>
  );
}

/**
 * An {@link EditableFieldRow} before its value is known: the real label and
 * hint (static copy, so they wrap exactly as the ready row does), then the
 * same control row — an empty disabled input, or a select-trigger-shaped box
 * holding a value placeholder — beside the pencil slot. It renders no value
 * and binds no mutation; route skeletons place it inside an `inert` card.
 *
 * `select` previews the select variant, whose value label wraps: it reserves
 * `narrowLines` 20px lines below `sm` (a long option label such as the
 * recommended maxmemory policy wraps there) and one line from `sm`.
 */
export function EditableFieldRowPreview({
  label,
  hint,
  select,
}: {
  label: string;
  hint?: ReactNode;
  select?: { narrowLines: 1 | 2 };
}) {
  return (
    <div className="space-y-2" data-slot="editable-field-row-preview">
      <div>
        <div className="flex items-center gap-2 text-sm font-medium">
          {label}
        </div>
        {hint != null && (
          <div className="text-muted-foreground mt-1 text-sm">{hint}</div>
        )}
      </div>
      <div
        className={cn(
          "flex items-center gap-2",
          select && "flex-wrap sm:flex-nowrap",
        )}
      >
        {select ? (
          <div
            data-slot="select-trigger-preview"
            className="border-input flex min-h-8 min-w-0 flex-1 items-center justify-between gap-2 rounded-md border px-3 py-2 text-sm opacity-50 shadow-xs"
          >
            <span className="min-w-0 flex-1">
              <span className="flex h-5 items-center">
                <Skeleton className="h-3.5 w-40 max-w-full" />
              </span>
              {select.narrowLines === 2 && (
                <span className="flex h-5 items-center sm:hidden">
                  <Skeleton className="h-3.5 w-24" />
                </span>
              )}
            </span>
            <ChevronDown className="text-muted-foreground size-4 shrink-0 opacity-50" />
          </div>
        ) : (
          <Input disabled aria-label={label} className="flex-1" />
        )}
        <Button size="icon" variant="ghost" disabled aria-hidden="true">
          <Pencil />
        </Button>
      </div>
    </div>
  );
}
