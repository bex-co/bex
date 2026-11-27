import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";

/**
 * The layout the environment settings cards share with their
 * pending frame, so a card and its preview cannot lay out differently. Kept
 * free of data and hooks: the route skeletons load it up front.
 */

/**
 * The body of Render's Permissions and Cross-Environment Connections cards:
 * the explanation on the left (above, on a narrow card), the two options and
 * their Edit / Cancel · Save on the right.
 */
export function ChoiceCardBody({
  explanation,
  choice,
  actions,
}: {
  explanation: ReactNode;
  choice: ReactNode;
  actions: ReactNode;
}) {
  return (
    <div className="grid gap-6 @3xl:grid-cols-[1fr_2fr] @3xl:gap-8">
      <div className="max-w-[35rem] space-y-4 text-sm text-muted-foreground">
        {explanation}
      </div>
      <div className="grid gap-6 self-start">
        {choice}
        {actions ? <div className="justify-self-end">{actions}</div> : null}
      </div>
    </div>
  );
}

/** One option of such a card: a bordered row, its radio, title and line. */
export function ChoiceOptionRow({
  selected = false,
  radio,
  title,
  icon: Icon,
  hint,
  htmlFor,
}: {
  selected?: boolean;
  /** The radio input, or its placeholder. */
  radio: ReactNode;
  title: ReactNode;
  icon?: LucideIcon;
  hint: ReactNode;
  /** The radio's id, which the row's label names. */
  htmlFor?: string;
}) {
  const Label = htmlFor ? "label" : "span";
  return (
    <div
      className={cn(
        "flex items-start gap-2.5 rounded-md border p-3",
        selected && "border-primary/60 bg-accent/40",
      )}
    >
      {radio}
      <Label
        htmlFor={htmlFor}
        className="flex flex-1 cursor-pointer flex-col gap-0.5 text-sm"
      >
        <span className="relative flex w-fit items-center gap-2 font-medium">
          {title}
          {Icon ? (
            <Icon
              aria-hidden="true"
              className="size-4 shrink-0 text-muted-foreground"
            />
          ) : null}
        </span>
        <span className="relative w-fit text-muted-foreground">{hint}</span>
      </Label>
    </div>
  );
}

/** Render's plan gate under a card: its sentence, and Upgrade. */
export function PlanGateRow({
  text,
  action,
}: {
  text: ReactNode;
  action: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border bg-muted/40 p-3 text-sm">
      <p className="relative">{text}</p>
      {action}
    </div>
  );
}

/** The radio's box, which the pending frame reserves. */
export const CHOICE_RADIO_CLASS = "mt-0.5 size-4 shrink-0";

/** Under an editing card's content: the rule and its Cancel / Save. */
export const EDIT_FOOTER_CLASS = "border-t px-5 pt-4";
