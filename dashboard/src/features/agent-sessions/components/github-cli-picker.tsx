import { GithubIcon } from "@/common/components/github-icon";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  GITHUB_CLI_CHOICES,
  githubCliChoiceLabel,
  githubCliOnLockKey,
  type GithubCliChoice,
  type GithubCliOnLock,
} from "@/features/agent-sessions/lib/github-cli";

export interface GithubCliPickerProps {
  value: GithubCliChoice;
  onChange: (choice: GithubCliChoice) => void;
  /** What inherit resolves to, shown beside it; null while unknown. */
  inherited: boolean | null;
  /** Why "on" cannot be chosen, shown under it; null when it can. */
  onLock: GithubCliOnLock | null;
  disabled?: boolean;
  /** A compact control in a composer toolbar, or a full-width form field. */
  variant?: "toolbar" | "field";
  id?: string;
  className?: string;
  /** A form field's error and help wiring. */
  invalid?: boolean;
  describedBy?: string;
}

/**
 * The `gh` binding control (w10/m561, Devin's security profile chosen in the
 * session start box's options or an automation's editor): inherit, on or off.
 * Inherit names what it resolves to, and "on" is disabled with the reason
 * where bex-api would refuse it: under the workspace's mandatory off, and for
 * a member who is not a workspace admin.
 */
export function GithubCliPicker({
  value,
  onChange,
  inherited,
  onLock,
  disabled,
  variant = "toolbar",
  id,
  className,
  invalid,
  describedBy,
}: GithubCliPickerProps) {
  const { t } = useTranslations();
  const toolbar = variant === "toolbar";
  const label = (choice: GithubCliChoice) =>
    githubCliChoiceLabel(choice, inherited, t);
  const shown = label(value);

  return (
    <Select
      value={value}
      onValueChange={(next) => onChange(next as GithubCliChoice)}
      disabled={disabled}
    >
      <SelectTrigger
        id={id}
        size={toolbar ? "sm" : "default"}
        // A form field is named and described by its own label.
        aria-label={toolbar ? t("agentEnvironment.githubCliLabel") : undefined}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        title={
          toolbar
            ? t("agentSessions.githubCliTitle", { choice: shown })
            : undefined
        }
        data-testid="github-cli-picker"
        className={cn(
          toolbar
            ? // The composer skeleton holds this width (detail-skeletons.tsx).
              "h-8 w-[9.5rem] gap-1.5 border-0 bg-transparent text-xs shadow-none"
            : "w-full",
          className,
        )}
      >
        {toolbar ? (
          <GithubIcon aria-hidden className="size-3.5 shrink-0" />
        ) : null}
        <SelectValue>
          <span className="truncate">{shown}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {GITHUB_CLI_CHOICES.map((choice) => {
          const lock = choice === "on" ? onLock : null;
          return (
            <SelectItem key={choice} value={choice} disabled={lock != null}>
              {lock ? (
                <span className="flex flex-col items-start">
                  {label(choice)}
                  <span className="text-muted-foreground text-xs">
                    {t(githubCliOnLockKey(lock))}
                  </span>
                </span>
              ) : (
                label(choice)
              )}
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}
