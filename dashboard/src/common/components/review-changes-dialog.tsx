import { Fragment, type ReactNode, type RefObject } from "react";
import { Loader2, Pencil, TriangleAlert } from "lucide-react";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";

export interface ReviewLineItem {
  id: string;
  label: string;
  /** The line's figures, each already worded: "1 CPU", "2 GB RAM". */
  values: string[];
  /** The line's monthly price, already worded ("$19.00/month"). */
  price: string;
  /** Returns to the control this line was chosen in; omitted, no Edit. */
  onEdit?: () => void;
  /** The Edit button's accessible name ("Edit compute plan"). */
  editLabel?: string;
}

export interface ReviewChangesDialogProps {
  open: boolean;
  /** The lead: "Review your compute and storage selections." */
  description: string;
  lineItems: ReviewLineItem[];
  /** The monthly total, already worded. */
  total: string;
  confirmLabel: string;
  /** What applying costs besides money (downtime, data loss). */
  warning?: ReactNode;
  /**
   * The change deletes data: confirm waits for this `sudo …` phrase and is
   * drawn as destructive, as every such confirmation is (dashboard AGENTS.md).
   */
  phrase?: string;
  /** The change is being applied: confirm spins, nothing closes the dialog. */
  pending?: boolean;
  /** Why the last confirm failed; the dialog stays open to show it. */
  error?: string | null;
  onConfirm: () => void;
  /** Cancel and Escape. */
  onClose: () => void;
  /** Where focus goes on close instead of the opener (ConfirmDialog's). */
  returnFocusRef?: RefObject<HTMLElement | null>;
}

/**
 * Render's one review before a compute or storage change applies (w3/m674,
 * its `BN` component; docs/render-artifacts/postgres-dashboard.md § Plan
 * review): "Apply changes?", the lead, each billed line with its figures, an
 * Edit back to its control and its monthly price, the Total, then a warning
 * and an error when there are any, Cancel and the change's own button. It is
 * the dashboard's ConfirmDialog, so focus, Escape and a phrase behave as in
 * every other confirmation.
 */
export function ReviewChangesDialog({
  open,
  description,
  lineItems,
  total,
  confirmLabel,
  warning,
  phrase,
  pending = false,
  error,
  onConfirm,
  onClose,
  returnFocusRef,
}: ReviewChangesDialogProps) {
  const { t } = useTranslations();
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(next) => {
        // Dismissing mid-flight would hide a change that still applies.
        if (!next && !pending) onClose();
      }}
      title={t("common.reviewChangesTitle")}
      description={description}
      confirmLabel={
        <>
          {pending ? <Loader2 className="animate-spin" /> : null}
          {confirmLabel}
        </>
      }
      destructive={phrase !== undefined}
      phrase={phrase}
      pending={pending}
      // A refused change says why here, so the dialog closes only when the
      // caller has applied it.
      closeOnConfirm={false}
      onConfirm={onConfirm}
      contentClassName="sm:max-w-xl"
      returnFocusRef={returnFocusRef}
    >
      <div className="flex flex-col gap-4 text-sm">
        {lineItems.map((line, index) => (
          <Fragment key={line.id}>
            {index > 0 ? <hr className="border-border" /> : null}
            <div
              data-slot="review-line"
              className="grid grid-cols-[1fr_auto] items-baseline gap-x-4 gap-y-1"
            >
              <span className="text-muted-foreground font-mono text-xs uppercase tracking-wide">
                {line.label}
              </span>
              <span className="col-start-1 row-start-2 flex flex-wrap items-center gap-x-3 gap-y-1">
                {line.values.map((value, i) => (
                  <span key={value} className="flex items-center gap-1.5">
                    <span className="font-medium">{value}</span>
                    {i === line.values.length - 1 && line.onEdit ? (
                      <button
                        type="button"
                        onClick={line.onEdit}
                        disabled={pending}
                        aria-label={line.editLabel}
                        title={line.editLabel}
                        className="text-muted-foreground hover:text-foreground focus-visible:outline-foreground relative cursor-pointer rounded-sm after:absolute after:-inset-3.5 after:content-[''] focus-visible:outline-2 focus-visible:outline-offset-1 disabled:pointer-events-none disabled:opacity-50"
                      >
                        <Pencil aria-hidden="true" className="size-4" />
                      </button>
                    ) : null}
                  </span>
                ))}
              </span>
              <span className="col-start-2 row-start-2 text-right font-semibold tabular-nums">
                {line.price}
              </span>
            </div>
          </Fragment>
        ))}
        <hr className="border-border" />
        <div className="flex items-baseline justify-between gap-x-4">
          <span>{t("common.reviewChangesTotal")}</span>
          <span className="font-semibold tabular-nums">{total}</span>
        </div>
      </div>
      {warning ? (
        <div className="flex items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm">
          <TriangleAlert
            aria-hidden="true"
            className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400"
          />
          <p>{warning}</p>
        </div>
      ) : null}
      {error ? (
        <p
          role="alert"
          aria-atomic="true"
          className="border-destructive/40 bg-destructive/10 text-destructive rounded-lg border px-4 py-3 text-sm"
        >
          {error}
        </p>
      ) : null}
    </ConfirmDialog>
  );
}
