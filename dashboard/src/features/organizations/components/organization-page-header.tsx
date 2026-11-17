import { Link } from "@tanstack/react-router";
import { PageHeader } from "@/common/components/page-frame";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton, SkeletonSpan } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import {
  ORGANIZATION_ROLE_LABEL_KEYS,
  ORGANIZATION_SECTIONS,
  type OrganizationRole,
  type OrganizationSection,
} from "@/features/organizations/types";

/** Each page's tab label and the description its header shows. */
const SECTION_KEYS: Record<
  OrganizationSection,
  { label: string; description: string }
> = {
  workspaces: {
    label: "organizations.navWorkspaces",
    description: "organizations.workspacesDescription",
  },
  members: {
    label: "organizations.navMembers",
    description: "organizations.membersDescription",
  },
  settings: {
    label: "organizations.navSettings",
    description: "organizations.settingsDescription",
  },
};

const TAB_CLASS =
  "-mb-px whitespace-nowrap border-b-2 px-3 py-2 text-sm transition-colors";

/**
 * The org's Workspaces, Members and Settings pages (Render's org left pane,
 * here a row under the header since the dashboard has one rail). The current
 * page is `aria-current="page"`.
 */
function OrganizationNav({
  organizationId,
  section,
}: {
  organizationId: string;
  section: OrganizationSection;
}) {
  const { t } = useTranslations();
  return (
    <nav
      aria-label={t("organizations.navLabel")}
      className="flex gap-1 overflow-x-auto border-b"
    >
      {ORGANIZATION_SECTIONS.map((item) => {
        const active = item === section;
        return (
          <Link
            key={item}
            to={`/org/$orgId/${item}`}
            params={{ orgId: organizationId }}
            aria-current={active ? "page" : undefined}
            className={cn(
              TAB_CLASS,
              active
                ? "border-foreground font-medium text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {t(SECTION_KEYS[item].label)}
          </Link>
        );
      })}
    </nav>
  );
}

/**
 * Every org page's header: the org's name with the caller's role, the page's
 * own description, and the org navigation. `pending` draws the name and role
 * as bars of their ready boxes (the route skeletons); the description and the
 * navigation are static copy, so they hold still.
 */
export function OrganizationPageHeader({
  organizationId,
  section,
  name,
  role,
  pending = false,
}: {
  organizationId: string;
  section: OrganizationSection;
  /** The org's name; null when it could not be read. */
  name?: string | null;
  role?: OrganizationRole | null;
  pending?: boolean;
}) {
  const { t } = useTranslations();
  const title = pending ? (
    // The h1's 28px line, as a bar the width of a typical org name.
    <span className="relative inline-block h-7 w-48 max-w-full align-top">
      <SkeletonSpan />
    </span>
  ) : (
    (name ?? t("organizations.untitled"))
  );
  const roleBadge = pending ? (
    // A secondary Badge's box: text-xs line + py-0.5 + border.
    <Skeleton className="h-[1.375rem] w-16" />
  ) : role ? (
    <Badge variant="secondary">{t(ORGANIZATION_ROLE_LABEL_KEYS[role])}</Badge>
  ) : null;

  return (
    <div className="space-y-4">
      <PageHeader
        title={title}
        titleAddon={roleBadge}
        description={t(SECTION_KEYS[section].description)}
      />
      <OrganizationNav organizationId={organizationId} section={section} />
    </div>
  );
}
