import { useId, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { Plus, Settings } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import {
  PROJECT_HEADER_CLASS,
  PROJECT_HEADER_ROW_CLASS,
} from "@/features/projects/lib/project-header-layout";
import { UpgradePlanLink } from "@/features/workspaces/components/upgrade-plan-link";

/** bex's page on projects and environments, Render's `/docs/projects`. */
const PROJECTS_DOCS_URL = "https://bex.co/docs/platform/projects-environments";

/**
 * The project page's header, as Render draws it (w1/m682): a "Project"
 * eyebrow over the name, which links to Project Settings with its Name card
 * already editing (`?edit=name`, a gear beside it, "Edit project name" for a
 * screen reader), and + Add environment on the right. The loading preview
 * (`ProjectOverviewPageSkeleton`) uses the same classes.
 */
export function ProjectPageHeader({
  projectId,
  name,
  addEnvironment,
}: {
  projectId: string;
  name: string;
  addEnvironment: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <header className={PROJECT_HEADER_CLASS}>
      <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {t("projects.eyebrow")}
      </p>
      <div className={PROJECT_HEADER_ROW_CLASS}>
        <Link
          to="/project/$projectId/settings"
          params={{ projectId }}
          search={{ edit: "name" }}
          className="group inline-flex min-w-0 items-center gap-2 rounded-md"
        >
          <h1 className="truncate text-xl font-semibold group-hover:underline">
            {name}
          </h1>
          <Settings
            aria-hidden="true"
            className="size-4 shrink-0 text-muted-foreground"
          />
          <span className="sr-only">{t("projects.editProjectName")}</span>
        </Link>
        {addEnvironment}
      </div>
    </header>
  );
}

/**
 * + Add environment. Below the plan's environments per project (two on
 * Hobby, ADR030) it opens the new-environment dialog; at the limit it shows
 * the count against the limit and opens Render's paywall, "Get more
 * environments with Pro", instead; for a role that cannot create, it is
 * disabled with the role's reason.
 */
export function AddEnvironmentButton({
  count,
  limit,
  reason,
  onClick,
}: {
  count: number;
  /** The plan's environments per project; `null` when unlimited or unknown. */
  limit: number | null;
  reason?: string;
  onClick: () => void;
}) {
  const { t } = useTranslations();
  const label = (
    <>
      <Plus />
      {t("projects.addEnvironment")}
    </>
  );
  if (reason !== undefined) {
    return (
      <PermissionTooltip reason={reason}>
        <Button variant="outline" size="sm" disabled>
          {label}
        </Button>
      </PermissionTooltip>
    );
  }
  if (limit === null || count < limit) {
    return (
      <Button variant="outline" size="sm" onClick={onClick}>
        {label}
      </Button>
    );
  }
  return <EnvironmentLimitPaywall count={count} limit={limit} label={label} />;
}

/** Render's paywall at the plan's environment limit, on the button's count. */
function EnvironmentLimitPaywall({
  count,
  limit,
  label,
}: {
  count: number;
  limit: number;
  label: ReactNode;
}) {
  const { t } = useTranslations();
  const titleId = useId();
  const plan = t("workspaces.planProName");
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm">
          {label}
          <span className="tabular-nums text-muted-foreground">
            {count}/{limit}
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        aria-labelledby={titleId}
        className="w-80 space-y-3"
      >
        <Badge variant="secondary">{plan}</Badge>
        <div className="space-y-1">
          <p id={titleId} className="font-semibold">
            {t("projects.environmentLimitTitle", { plan })}
          </p>
          <p className="text-sm text-muted-foreground">
            {t("projects.environmentLimitBody", { plan })}
          </p>
        </div>
        <div className="flex gap-2">
          <Button asChild size="sm">
            <UpgradePlanLink plan="pro">
              {t("projects.environmentLimitUpgrade")}
            </UpgradePlanLink>
          </Button>
          <Button asChild variant="outline" size="sm">
            <a href={PROJECTS_DOCS_URL} target="_blank" rel="noreferrer">
              {t("projects.environmentLimitLearnMore")}
            </a>
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
