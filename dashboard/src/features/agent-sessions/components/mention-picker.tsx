import { BookMarked, Bot, Check, Lock } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { mentionOptionId } from "@/features/agent-sessions/lib/mention";
import type { MentionOption } from "@/features/agent-sessions/lib/mention";

export interface MentionPickerProps {
  /** DOM id of the listbox (aria-controls target) + per-option id base. */
  idBase: string;
  /** The already-filtered options, in render order (composer owns filtering). */
  options: MentionOption[];
  /** Index of the keyboard-highlighted option. */
  highlight: number;
  /** Shown when `options` is empty (no source rows vs. no fuzzy matches). */
  emptyText: string;
  onHighlight: (index: number) => void;
  onSelect: (option: MentionOption) => void;
}

/**
 * The two-level `@` mention popup (w3/m45 t002, Devin's picker shape): level 1
 * lists the categories (Repositories / Sessions with one-line descriptions);
 * level 2 lists that category's fuzzy-filtered items — repo rows as name +
 * owner subtitle, session rows as task title + phase. Presentation-only: the
 * composer owns the state machine (token insertion, caret tracking, filtering,
 * keyboard nav) and passes the flattened option list down. For a highlighted
 * repo, a readiness preview footer shows owner/name, the GitHub-App-connected
 * state (every row comes from the connected installation's `repos` query), and
 * the default branch.
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
          options.map((option, index) => (
            <MentionOptionRow
              key={optionKey(option)}
              id={mentionOptionId(idBase, index)}
              option={option}
              active={index === highlight}
              onHighlight={() => onHighlight(index)}
              onSelect={() => onSelect(option)}
            />
          ))
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

function optionKey(option: MentionOption): string {
  switch (option.kind) {
    case "category":
      return `category:${option.category}`;
    case "repo":
      return `repo:${option.repo.fullName}`;
    case "session":
      return `session:${option.session.id}`;
  }
}

function MentionOptionRow({
  id,
  option,
  active,
  onHighlight,
  onSelect,
}: {
  id: string;
  option: MentionOption;
  active: boolean;
  onHighlight: () => void;
  onSelect: () => void;
}) {
  const { t } = useTranslations();

  let icon: React.ReactNode;
  let title: string;
  let subtitle: string;
  let privateBadge = false;
  switch (option.kind) {
    case "category": {
      icon =
        option.category === "repos" ? (
          <BookMarked className="size-4" aria-hidden />
        ) : (
          <Bot className="size-4" aria-hidden />
        );
      title =
        option.category === "repos"
          ? t("agentSessions.mentionCategoryRepos")
          : t("agentSessions.mentionCategorySessions");
      subtitle =
        option.category === "repos"
          ? t("agentSessions.mentionCategoryReposDesc")
          : t("agentSessions.mentionCategorySessionsDesc");
      break;
    }
    case "repo": {
      const [owner, ...rest] = option.repo.fullName.split("/");
      icon = <BookMarked className="size-4" aria-hidden />;
      title = rest.join("/") || option.repo.fullName;
      subtitle = owner ?? "";
      privateBadge = option.repo.private;
      break;
    }
    case "session": {
      icon = <Bot className="size-4" aria-hidden />;
      title = option.session.agentConfig.task || option.session.id;
      subtitle = t(`agentSessions.phase.${option.session.phase}`);
      break;
    }
  }

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
      <span className="text-muted-foreground mt-0.5 shrink-0">{icon}</span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5">
          <span className="truncate font-medium">{title}</span>
          {privateBadge ? (
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
