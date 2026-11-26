import { useId, useState, type ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ArrowUpRight, ChevronDown, Loader2 } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { HydrationFieldset } from "@/common/components/hydration-fieldset";
import {
  Select,
  SelectContent,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Skeleton } from "@/common/components/ui/skeleton";
import type { ComputePlanTier } from "@/common/components/compute-plan-list";
import { useTranslations } from "@/common/hooks/use-translations";
import { planPriceLabel } from "@/common/lib/plan-list";
import { cn } from "@/common/lib/utils/utils";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

/**
 * Render's create-page template, shared by every bex create page (services,
 * Postgres, Key Value, Blueprints): one field per row — the label, an
 * "Optional" mark and a one-line description in the left third, the control
 * in the right two thirds — stacked when the body column is narrow (live
 * captures of render.com/web/new, /new/database, /new/redis, 2026-11-21).
 * Rows measure the page body's container, not the viewport, so an open
 * sidebar at 768 px stacks them too.
 */
export function CreateForm({ children }: { children: ReactNode }) {
  return (
    <HydrationFieldset>
      <div className="space-y-8">{children}</div>
    </HydrationFieldset>
  );
}

/** The row grid: label column beside control column from `@2xl` up. */
export const CREATE_FIELD_GRID_CLASS =
  "grid gap-x-10 gap-y-2 @2xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]";

export function CreateField({
  label,
  htmlFor,
  optional = false,
  required = false,
  description,
  descriptionId,
  children,
}: {
  label: ReactNode;
  /** The control the label names; omit for a group of controls. */
  htmlFor?: string;
  optional?: boolean;
  /** A field Submit waits for, marked as plainly as the optional ones. */
  required?: boolean;
  description?: ReactNode;
  /** The description's id, for the control's `aria-describedby`. */
  descriptionId?: string;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <div className={CREATE_FIELD_GRID_CLASS}>
      <div className="min-w-0 space-y-1">
        <div className="flex flex-wrap items-baseline gap-x-2">
          {htmlFor ? (
            <Label htmlFor={htmlFor}>{label}</Label>
          ) : (
            <p className="text-sm leading-none font-medium">{label}</p>
          )}
          {optional ? (
            <span className="text-muted-foreground text-sm">
              {t("common.createOptional")}
            </span>
          ) : required ? (
            <span className="text-foreground/70 text-sm">
              {t("common.createRequired")}
            </span>
          ) : null}
        </div>
        {description ? (
          <div
            id={descriptionId}
            className="text-muted-foreground max-w-prose text-sm"
          >
            {description}
          </div>
        ) : null}
      </div>
      <div className="min-w-0 space-y-2">{children}</div>
    </div>
  );
}

/**
 * A text input row. `description` is the row's standing copy in the label
 * column; `hint` is copy that follows the value (a schedule preview, a
 * required-field reminder) and sits under the input, where `error` replaces
 * it. The input names whichever applies to assistive technology. A `disabled`
 * row says why in its `hint`.
 */
export function CreateTextField({
  id,
  label,
  value,
  onChange,
  placeholder,
  description,
  hint,
  error,
  optional,
  required,
  mono = false,
  disabled = false,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  description?: ReactNode;
  hint?: string;
  error?: string;
  optional?: boolean;
  required?: boolean;
  /** Commands and paths read in the monospace face, as on Render. */
  mono?: boolean;
  disabled?: boolean;
}) {
  const descriptionId = useId();
  const noteId = useId();
  const describedBy =
    [description ? descriptionId : null, error || hint ? noteId : null]
      .filter(Boolean)
      .join(" ") || undefined;
  return (
    <CreateField
      label={label}
      htmlFor={id}
      optional={optional}
      required={required}
      description={description}
      descriptionId={descriptionId}
    >
      <Input
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete="off"
        aria-invalid={Boolean(error)}
        aria-describedby={describedBy}
        className={cn(CREATE_INPUT_CLASS, mono && "font-mono")}
      />
      {error ? (
        <p id={noteId} className="text-destructive text-sm" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p id={noteId} className="text-muted-foreground text-sm">
          {hint}
        </p>
      ) : null}
    </CreateField>
  );
}

/** Inputs on a create page: a 44 px touch target on a phone, the
 *  dashboard's 36 px from `sm` up. */
export const CREATE_INPUT_CLASS = "h-11 sm:h-9";

/** A create row's Switch: the 32×18 control keeps its look, its tap target
 *  grows to 44 px through an invisible ring (the button's own ::after). */
export const CREATE_SWITCH_CLASS =
  "relative after:absolute after:-inset-3 after:content-['']";

/** A create page's select trigger, at the same heights: the trigger sizes
 *  itself through `data-[size=default]:h-9`, which a plain height loses to. */
const CREATE_SELECT_HEIGHT_CLASS =
  "data-[size=default]:h-11 sm:data-[size=default]:h-9";
export const CREATE_SELECT_CLASS = `w-full ${CREATE_SELECT_HEIGHT_CLASS}`;

/** A select row; `children` are its `SelectItem`s. */
export function CreateSelectField({
  id,
  label,
  description,
  value,
  onValueChange,
  children,
}: {
  id: string;
  label: string;
  description?: ReactNode;
  value: string;
  onValueChange: (value: string) => void;
  children: ReactNode;
}) {
  return (
    <CreateField label={label} htmlFor={id} description={description}>
      {/* Its own box: before hydration Radix adds a hidden native <select>
          beside the trigger, which must not take the row's gap. */}
      <div>
        <Select value={value} onValueChange={onValueChange}>
          <SelectTrigger id={id} className={CREATE_SELECT_CLASS}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>{children}</SelectContent>
        </Select>
      </div>
    </CreateField>
  );
}

/** The Project row's two selects, ready and pending. */
export const PROJECT_SELECTS_GRID_CLASS =
  "grid grid-cols-1 items-center gap-2 @2xl:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]";

/**
 * A full-width card inside the form, for a group with its own table or list
 * (Render's Environment Variables card).
 */
export function CreateSection({
  title,
  description,
  children,
}: {
  title: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  const titleId = useId();
  return (
    <section
      aria-labelledby={titleId}
      className="space-y-4 rounded-lg border p-4 sm:p-6"
    >
      <div className="space-y-1">
        <h2 id={titleId} className="text-base font-semibold">
          {title}
        </h2>
        {description ? (
          <div className="text-muted-foreground max-w-prose text-sm">
            {description}
          </div>
        ) : null}
      </div>
      {children}
    </section>
  );
}

/**
 * Render's collapsed "Advanced" card: the settings most services never
 * change. `forceOpen` holds it open while a field inside is invalid, so the
 * reason Deploy is blocked is never folded away.
 */
export function CreateAdvanced({
  children,
  forceOpen = false,
}: {
  children: ReactNode;
  forceOpen?: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const shown = open || forceOpen;
  const contentId = useId();
  return (
    <section className="rounded-lg border">
      <h2>
        <button
          type="button"
          aria-expanded={shown}
          aria-controls={contentId}
          onClick={() => setOpen(!shown)}
          className={cn(
            "flex min-h-12 w-full items-center gap-2 rounded-lg px-4 text-left text-base font-semibold transition-colors",
            "hover:bg-muted/60 focus-visible:outline-ring focus-visible:outline-2 focus-visible:outline-offset-2",
            shown && "bg-muted/60 rounded-b-none",
          )}
        >
          <ChevronDown
            aria-hidden
            className={cn(
              "size-4 shrink-0 transition-transform motion-reduce:transition-none",
              !shown && "-rotate-90",
            )}
          />
          {t("common.createAdvanced")}
        </button>
      </h2>
      {shown ? (
        <div id={contentId} className="space-y-8 border-t p-4 sm:p-6">
          {children}
        </div>
      ) : null}
    </section>
  );
}

/**
 * The sticky action bar under every create page (PageFrame's `footer`):
 * the type-specific submit first, Cancel, then what the choice costs, as on
 * Render. While Submit is held, `blocked` says why in words; with a
 * `fieldId`, the reason is a button that takes the user (and focus) to it.
 */
export function CreateActionBar({
  submitLabel,
  onSubmit,
  canSubmit,
  busy,
  cancelLabel,
  onCancel,
  summary,
  blocked,
}: {
  submitLabel: string;
  onSubmit: () => void;
  canSubmit: boolean;
  busy: boolean;
  cancelLabel: string;
  onCancel: () => void;
  summary?: ReactNode;
  blocked?: { reason: string; fieldId?: string | null } | null;
}) {
  const reasonId = useId();
  const held = canSubmit || busy ? null : blocked;
  const blockedReason = held?.reason;
  const blockedFieldId = held?.fieldId;
  return (
    <HydrationFieldset>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <Button
          size="lg"
          className="h-11 px-5 sm:h-10"
          onClick={onSubmit}
          disabled={!canSubmit}
          aria-describedby={blockedReason ? reasonId : undefined}
        >
          {busy ? <Loader2 className="animate-spin" /> : null}
          {submitLabel}
        </Button>
        <Button
          variant="outline"
          size="lg"
          className="h-11 sm:h-10"
          onClick={onCancel}
          disabled={busy}
        >
          {cancelLabel}
        </Button>
        {/* What the choice costs, then — beside it, where the eye already is
            — why Submit is held. */}
        {summary || blockedReason ? (
          <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5 text-sm">
            {summary}
            {blockedReason && blockedFieldId ? (
              <button
                type="button"
                id={reasonId}
                // The text keeps its line; the tap target reaches 44 px.
                className="text-muted-foreground hover:text-foreground focus-visible:outline-ring relative rounded-sm text-left underline decoration-dotted underline-offset-4 after:absolute after:inset-x-0 after:-inset-y-3 after:content-[''] focus-visible:outline-2 focus-visible:outline-offset-2"
                onClick={() => {
                  const field = document.getElementById(blockedFieldId);
                  field?.scrollIntoView?.({ block: "center" });
                  field?.focus({ preventScroll: true });
                }}
              >
                {blockedReason}
              </button>
            ) : blockedReason ? (
              <p id={reasonId} className="text-muted-foreground">
                {blockedReason}
              </p>
            ) : null}
          </div>
        ) : null}
      </div>
    </HydrationFieldset>
  );
}

/** A create the workspace's plan refused: bex-api's message, and the way to
 *  a bigger plan when one would allow it. */
export function CreateCapLimitAlert({
  title,
  message,
  upgradeLabel,
  showUpgrade = true,
}: {
  title: string;
  message: string;
  upgradeLabel: string;
  showUpgrade?: boolean;
}) {
  const navigate = useNavigate();
  return (
    <Alert variant="destructive">
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription className="flex flex-col gap-2">
        <span>{message}</span>
        {showUpgrade ? (
          <Button
            variant="outline"
            size="sm"
            className="self-start"
            onClick={() =>
              void navigate({
                to: "/workspace/settings",
                search: { plan: "change" },
              })
            }
          >
            <ArrowUpRight className="size-3.5" />
            {upgradeLabel}
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

/** The plan a create page's action bar prices: Render's "$7 / month
 *  0.5 CPU 512 MB RAM". */
export function PlanSummary({
  plan,
  runTimePricing = false,
  loading = false,
}: {
  plan:
    | Pick<ComputePlanTier, "cpu" | "memory" | "monthlyUsd" | "usdPerMinute">
    | undefined;
  runTimePricing?: boolean;
  /** The catalog is still loading: hold the summary's place, so the bar's
   *  hint and height don't move when the price arrives. */
  loading?: boolean;
}) {
  const { t } = useTranslations();
  if (!plan) {
    return loading ? (
      // As wide as "$0.00/month  0.1 CPU  128 MB RAM", so the hint beside
      // it wraps (or not) exactly as it will once the price arrives.
      <span aria-hidden className="flex h-5 items-center">
        <Skeleton className="h-3.5 w-60" />
      </span>
    ) : null;
  }
  const price = planPriceLabel(plan, runTimePricing, t);
  return (
    <span className="text-muted-foreground flex flex-wrap items-baseline gap-x-3 gap-y-1">
      {price ? (
        <span className="text-foreground font-semibold tabular-nums">
          {price}
        </span>
      ) : null}
      <span>{formatInstanceCPU(plan.cpu)}</span>
      <span>{formatInstanceMemory(plan.memory)} RAM</span>
    </span>
  );
}

/**
 * A create row's pending shape (route skeletons): the label line, its
 * description lines, and the control — an input by default.
 */
export function CreateFieldSkeleton({
  descriptionLines = 1,
  children,
}: {
  descriptionLines?: number;
  children?: ReactNode;
}) {
  return (
    <div className={CREATE_FIELD_GRID_CLASS}>
      <div className="min-w-0 space-y-1">
        <div className="flex h-3.5 items-center">
          <Skeleton className="h-3.5 w-24" />
        </div>
        {Array.from({ length: descriptionLines }, (_, index) => (
          <div key={index} className="flex h-5 items-center">
            <Skeleton
              className={cn(
                "h-3.5",
                index === descriptionLines - 1 ? "w-3/5" : "w-full",
              )}
            />
          </div>
        ))}
      </div>
      <div className="min-w-0 space-y-2">
        {children ?? (
          <Skeleton className={cn(CREATE_INPUT_CLASS, "w-full rounded-md")} />
        )}
      </div>
    </div>
  );
}

/** The shared Project row's selects, pending. */
export function ProjectSelectsSkeleton() {
  return (
    <div className={PROJECT_SELECTS_GRID_CLASS}>
      <Skeleton className={cn(CREATE_INPUT_CLASS, "w-full rounded-md")} />
      <span className="hidden w-[1.125rem] @2xl:block" />
      <Skeleton className={cn(CREATE_INPUT_CLASS, "w-full rounded-md")} />
    </div>
  );
}

/** A Switch row's control, pending (the Switch's own 32×18 box). */
export function SwitchSkeleton() {
  return <Skeleton className="h-[1.15rem] w-8 rounded-full" />;
}

/**
 * The action bar, pending: the ready bar's own buttons, disabled (the frame
 * is inert), and a slot for the plan summary.
 */
export function CreateActionBarSkeleton({
  submitLabel,
  cancelLabel,
  summary = true,
}: {
  submitLabel: string;
  cancelLabel: string;
  summary?: boolean;
}) {
  return (
    <CreateActionBar
      submitLabel={submitLabel}
      onSubmit={() => {}}
      canSubmit={false}
      busy={false}
      cancelLabel={cancelLabel}
      onCancel={() => {}}
      summary={
        summary ? (
          <span className="flex h-5 items-center">
            <Skeleton className="h-3.5 w-44" />
          </span>
        ) : undefined
      }
    />
  );
}
