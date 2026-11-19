import { RadioDot } from "@/common/components/radio-dot";
import type { ButtonHTMLAttributes, Ref } from "react";
import { cn } from "@/common/lib/utils/utils";

/**
 * One choice of a Workspace Settings card radio group (Overlapping Deploy
 * Policy, w4/653; Workspace privacy, w7/m464): its label over one sentence.
 * A skeleton renders the same button unchecked and disabled (`pending`), its
 * text transparent on a pulsing bar, so it wraps alike at every width and in
 * every language.
 */
export function SettingRadioOption({
  label,
  description,
  checked,
  pending = false,
  buttonRef,
  ...props
}: {
  label: string;
  description: string;
  checked: boolean;
  pending?: boolean;
  buttonRef?: Ref<HTMLButtonElement>;
} & ButtonHTMLAttributes<HTMLButtonElement>) {
  const text = pending
    ? "bg-accent animate-pulse rounded-md box-decoration-clone text-transparent motion-reduce:animate-none"
    : "";
  return (
    <button
      {...props}
      ref={buttonRef}
      type="button"
      role="radio"
      aria-checked={checked}
      className={cn(
        "flex items-start gap-3 rounded-lg border p-3 text-left transition-colors disabled:cursor-not-allowed",
        !pending && "disabled:opacity-60",
        checked
          ? "border-primary ring-1 ring-primary"
          : "border-border enabled:hover:border-muted-foreground/50",
      )}
    >
      <RadioDot checked={checked} className="mt-0.5" />
      <span>
        <span className="block text-sm font-medium">
          <span className={text}>{label}</span>
        </span>
        <span className="text-muted-foreground block text-xs">
          <span className={text}>{description}</span>
        </span>
      </span>
    </button>
  );
}
