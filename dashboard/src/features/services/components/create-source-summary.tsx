import { Pencil } from "lucide-react";
import { GithubIcon } from "@/common/components/github-icon";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import type { RepoView } from "@/features/services/hooks/use-repos";

/**
 * The chosen repository, folded into one row once picked, with Edit to
 * reopen the picker — Render's create form does the same after "Connect"
 * (live capture 2026-11-21).
 */
export function CreateSourceSummary({
  repo,
  branch,
  onEdit,
}: {
  repo: RepoView;
  branch: string;
  onEdit: () => void;
}) {
  const { t } = useTranslations();
  const [owner, ...rest] = repo.fullName.split("/");
  return (
    <div className="flex min-h-12 items-center gap-3 rounded-lg border py-1.5 pr-1.5 pl-4">
      <GithubIcon className="text-muted-foreground size-4 shrink-0" />
      <p className="min-w-0 flex-1 truncate text-sm">
        <span className="text-muted-foreground">{owner}</span>
        {rest.length > 0 ? (
          <>
            <span className="text-muted-foreground"> / </span>
            <span className="font-medium">{rest.join("/")}</span>
          </>
        ) : null}
        {branch ? (
          <span className="text-muted-foreground ml-2 font-mono text-xs">
            {branch}
          </span>
        ) : null}
      </p>
      <Button
        type="button"
        variant="ghost"
        className="h-11 shrink-0 sm:h-9"
        onClick={onEdit}
        aria-label={t("services.createSourceEditLabel")}
      >
        <Pencil />
        {t("services.createSourceEdit")}
      </Button>
    </div>
  );
}
