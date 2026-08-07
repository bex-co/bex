import { BookMarked, Bot, Check, Lock } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  CATEGORY_META,
  mentionOptionId,
} from "@/features/agent-sessions/lib/mention";
import type {
  MentionCategory,
  MentionOption,
} from "@/features/agent-sessions/lib/mention";
import { sessionTitle } from "@/features/agent-sessions/lib/mapper";

export interface MentionPickerProps {
  /** DOM id of the listbox (aria-controls target) + per-option id base. */
  idBase: string;
  /** The already-filtered options, in render order (composer owns filtering). */
  options: MentionOption[];
  highlight: number;
  /** Shown when `options` is empty (no source rows vs. no fuzzy matches). */
  emptyText: string;
  onHighlight: (index: number) => void;
  onSelect: (option: MentionOption) => void;
}

const CATEGORY_ICONS: Record<MentionCategory, LucideIcon> = {
  repos: BookMarked,
  sessions: Bot,
};

interface OptionDescription {
  key: string;
  Icon: LucideIcon;
  title: string;
  subtitle: string;
  isPrivate: boolean;
}

function describeOption(
  option: MentionOption,
  t: ReturnType<typeof useTranslations>["t"],
): OptionDescription {
  switch (option.kind) {
    case "category": {
      const meta = CATEGORY_META[option.category];
      return {
        key: `category:${option.category}`,
        Icon: CATEGORY_ICONS[option.category],
        title: t(meta.labelKey),
        subtitle: t(meta.descKey),
        isPrivate: false,
      };
    }
    case "repo": {
      const [owner, ...rest] = option.repo.fullName.split("/");
      return {
        key: `repo:${option.repo.fullName}`,
        Icon: BookMarked,
        title: rest.join("/") || option.repo.fullName,
        subtitle: owner ?? "",
        isPrivate: option.repo.private,
      };
    }
    case "session":
      return {
        key: `session:${option.session.id}`,
        Icon: Bot,
        title: sessionTitle(option.session),
        subtitle: t(`agentSessions.phase.${option.session.phase}`),
        isPrivate: false,
      };
  }
}

/**
 * The two-level `@` mention popup (Devin's picker shape). Presentation-only:
 * the composer owns the state machine and hands down the flattened, already
 * filtered option list. For a highlighted repo, a readiness preview footer
 * shows the GitHub-App-connected state (every row comes from the connected
 * installation's `repos` query) and the default branch.
 */
export function MentionPicker({
  idBase,
  options,
  highlight,
  emptyText,
  onHighlight,
  onSelect,
}: MentionPickerProps) {
  const { t } = useTranslations();
  const highlighted = options[highlight];
  const footerRepo = highlighted?.kind === "repo" ? highlighted.repo : null;

  return (
    <div className="bg-popover text-popover-foreground absolute inset-x-0 top-full z-30 mt-1.5 overflow-hidden rounded-md border shadow-md">
      <div
        role="listbox"
        id={idBase}
        aria-label={t("agentSessions.mentionButton")}
        className="max-h-64 overflow-y-auto p-1"
      >
        {options.length === 0 ? (
          <p className="text-muted-foreground px-2 py-2 text-sm">{emptyText}</p>
        ) : (
          options.map((option, index) => {
            const description = describeOption(option, t);
            return (
              <MentionOptionRow
                key={description.key}
                id={mentionOptionId(idBase, index)}
                description={description}
                active={index === highlight}
                onHighlight={() => onHighlight(index)}
                onSelect={() => onSelect(option)}
              />
            );
          })
        )}
      </div>

      {footerRepo ? (
        <div className="text-muted-foreground space-y-0.5 border-t px-3 py-2 text-xs">
          <p className="text-foreground font-medium">{footerRepo.fullName}</p>
          <p className="flex items-center gap-1">
            <Check className="size-3 text-emerald-600" aria-hidden />
            {t("agentSessions.mentionConnected")}
          </p>
          <p>
            {t("agentSessions.mentionDefaultBranch", {
              branch: footerRepo.defaultBranch,
            })}
          </p>
        </div>
      ) : null}
    </div>
  );
}

function MentionOptionRow({
  id,
  description,
  active,
  onHighlight,
  onSelect,
}: {
  id: string;
  description: OptionDescription;
  active: boolean;
  onHighlight: () => void;
  onSelect: () => void;
}) {
  const { Icon, title, subtitle, isPrivate } = description;
  return (
    <button
      type="button"
      role="option"
      id={id}
      aria-selected={active}
      className={cn(
        "flex w-full items-start gap-2 rounded-sm px-2 py-1.5 text-left text-sm",
        active && "bg-accent text-accent-foreground",
      )}
      // preventDefault keeps focus in the composer textarea through the click.
      onMouseDown={(event) => event.preventDefault()}
      onMouseEnter={onHighlight}
      onClick={onSelect}
    >
      <span className="text-muted-foreground mt-0.5 shrink-0">
        <Icon className="size-4" aria-hidden />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5">
          <span className="truncate font-medium">{title}</span>
          {isPrivate ? (
            <Lock
              className="text-muted-foreground size-3 shrink-0"
              aria-hidden
            />
          ) : null}
        </span>
        {subtitle ? (
          <span className="text-muted-foreground block truncate text-xs">
            {subtitle}
          </span>
        ) : null}
      </span>
    </button>
  );
}
