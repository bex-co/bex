import { Link, useParams, useRouterState } from "@tanstack/react-router";
import {
  Bell,
  Bot,
  Box,
  CalendarClock,
  Network,
  Boxes,
  Building2,
  Check,
  ChevronDown,
  ChevronRight,
  CreditCard,
  Database,
  FolderKanban,
  Globe2,
  KeyRound,
  Layers,
  Package,
  Plus,
  Settings,
  UserRound,
  Webhook,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { cn } from "@/common/lib/utils/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { ENVIRONMENT_SETTINGS_ROUTE_ID } from "@/features/environments/lib/settings";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useServer } from "@/features/services/hooks/use-server";
import { useCachedServiceName } from "@/features/services/hooks/use-cached-service-type";
import { useServices } from "@/features/services/hooks/use-services";
import { useDatabase } from "@/features/databases/hooks/use-database";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { useEnvGroup } from "@/features/env-groups/hooks/use-env-groups";
import { useBuildSource } from "@/features/build-sources/hooks/use-build-source";
import { useBlueprint } from "@/features/blueprints/hooks/use-blueprint";
import { canonicalBlueprintLinkId } from "@/features/blueprints/lib/link-id";
import {
  CREATE_TYPE_ITEMS,
  currentCreateType,
  type CreateTypeItem,
} from "@/common/components/dashboard-layout/create-type-items";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { Skeleton } from "@/common/components/ui/skeleton";

type DashboardParams = {
  agentSessionId?: string;
  serviceId?: string;
  projectId?: string;
  environmentId?: string;
  databaseId?: string;
  keyValueId?: string;
  groupId?: string;
  buildSourceId?: string;
  blueprintId?: string;
  webhookId?: string;
};

/** Contextual hierarchy shown on the left of every dashboard topbar. */
export function DashboardBreadcrumbs() {
  // The location the page on screen rendered, not one still navigating: the
  // trail changes together with the body, never ahead of it.
  const pathname = useRouterState({
    select: (state) => (state.resolvedLocation ?? state.location).pathname,
  });
  const params = useParams({ strict: false }) as DashboardParams;
  const onEnvironmentSettings = useRouterState({
    select: (state) =>
      state.matches.some(
        (match) => match.routeId === ENVIRONMENT_SETTINGS_ROUTE_ID,
      ),
  });
  const createType = currentCreateType(pathname);
  if (createType) {
    return <CreateTypeBreadcrumbs current={createType} />;
  }
  if (params.serviceId) {
    return <ServiceBreadcrumbs serviceId={params.serviceId} />;
  }
  if (params.projectId) {
    // An environment's settings page adds the environment, as Render's
    // Project › Environment trail does (w1/m683); the project page at an
    // environment's URL keeps the project alone, as Render's does.
    const environmentId = onEnvironmentSettings
      ? params.environmentId
      : undefined;
    return (
      <ProjectBreadcrumbs
        projectId={params.projectId}
        environmentId={environmentId}
      />
    );
  }
  if (params.databaseId) {
    return <DatabaseBreadcrumbs id={params.databaseId} />;
  }
  if (params.keyValueId) {
    return <KeyValueBreadcrumbs id={params.keyValueId} />;
  }
  if (params.groupId) {
    return <EnvGroupBreadcrumbs id={params.groupId} />;
  }
  if (params.buildSourceId) {
    return <BuildSourceBreadcrumbs id={params.buildSourceId} />;
  }
  if (params.blueprintId) {
    return <BlueprintBreadcrumbs id={params.blueprintId} />;
  }
  return <PageBreadcrumb pathname={pathname} params={params} />;
}

function ServiceBreadcrumbs({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { service, loading } = useServer(serviceId, { poll: false });
  // While the read loads, the cached name the sidebar and the header's frame
  // show, not the id (w3/119).
  const pending = loading && service === null;
  const cachedName = useCachedServiceName(serviceId);
  const name = service?.name ?? (pending ? cachedName : null) ?? serviceId;
  const { services } = useServices({ poll: false });
  const { projects } = useProjects({ poll: false });
  const resolvedServiceId = service?.id ?? serviceId;
  const project = projects.find((item) =>
    item.serviceIds.includes(resolvedServiceId),
  );
  const projectServices = project
    ? services.filter((item) => project.serviceIds.includes(item.id))
    : services;
  const { environments } = useEnvironments(project?.id ?? null, {
    poll: false,
  });
  const environment = environments.find((item) =>
    item.serviceIds.includes(resolvedServiceId),
  );

  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      {/* Below `lg` the bar has room for the service crumb alone: beside a
          visible sidebar (768–1023) all three were cut to a letter or two
          ("s… › P… › ed…"), which named nothing. The service crumb keeps its
          switcher; project and environment return from `lg` up. */}
      <div className="hidden lg:contents">
        {project ? (
          <>
            <ProjectMenu currentId={project.id} projects={projects} />
            <BreadcrumbSeparator />
          </>
        ) : (
          <>
            <BreadcrumbLink
              to="/"
              icon={FolderKanban}
              label={t("common.navProjects")}
            />
            <BreadcrumbSeparator />
          </>
        )}
        {project && environment ? (
          <>
            <EnvironmentMenu
              projectId={project.id}
              currentId={environment.id}
              environments={environments}
            />
            <BreadcrumbSeparator />
          </>
        ) : null}
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="min-w-0 max-w-48 shrink gap-1 px-2 font-medium"
          >
            <Globe2 className="shrink-0" />
            <span className="truncate">{name}</span>
            <ChevronDown className="shrink-0" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-64">
          <DropdownMenuLabel>
            {t("common.topbarSwitchService")}
          </DropdownMenuLabel>
          {projectServices.map((item) => (
            <DropdownMenuItem key={item.id} asChild>
              <Link
                to="/services/$serviceId"
                params={{ serviceId: item.id }}
                className="min-w-0"
              >
                <Globe2 />
                <span className="truncate">{item.name}</span>
              </Link>
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem asChild>
            <Link to="/">
              <FolderKanban />
              {t("common.topbarAllResources")}
            </Link>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </nav>
  );
}

/**
 * A create page's trail, Render's "New Web Service ▾": the page's own heading
 * is the menu that switches to another resource type ("Choose a service
 * type", live capture 2026-11-21). A switch between service types keeps the
 * page mounted (one layout route) and the form's draft: it replaces the
 * history entry and keeps the Project/Environment context.
 */
function CreateTypeBreadcrumbs({ current }: { current: CreateTypeItem }) {
  const { t } = useTranslations();
  const CurrentIcon = current.icon;
  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      <BreadcrumbLink
        to="/"
        icon={FolderKanban}
        label={t("common.navProjects")}
      />
      <BreadcrumbSeparator />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="min-w-0 max-w-64 shrink gap-1.5 px-2 font-medium"
            aria-label={t("common.createTypeSwitcher", {
              type: t(current.titleKey),
            })}
          >
            <CurrentIcon className="shrink-0" />
            <span className="truncate">{t(current.titleKey)}</span>
            <ChevronDown className="shrink-0" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-60">
          {CREATE_TYPE_ITEMS.map((item) => {
            const { icon: Icon } = item;
            const selected = item.id === current.id;
            const keepDraft = item.service && current.service;
            return (
              <DropdownMenuItem key={item.id} asChild>
                <Link
                  to={item.to}
                  search={(prev: {
                    projectId?: string;
                    environmentId?: string;
                  }) =>
                    keepDraft
                      ? {
                          projectId: prev.projectId,
                          environmentId: prev.environmentId,
                        }
                      : {}
                  }
                  replace={keepDraft}
                  aria-current={selected ? "page" : undefined}
                  className={cn("min-h-9", selected && "font-medium")}
                >
                  <Icon />
                  {t(item.labelKey)}
                  {selected ? (
                    <Check aria-hidden className="ml-auto text-primary" />
                  ) : null}
                </Link>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </nav>
  );
}

// A datastore's name comes from the page's own detail hook, with polling off:
// the page polls that query and this leaf follows the cache, instead of a
// second timer drifting beside it (as ServiceBreadcrumbs does with useServer).
function DatabaseBreadcrumbs({ id }: { id: string }) {
  const { t } = useTranslations();
  const { database } = useDatabase(id, { poll: false });
  return (
    <DatastoreBreadcrumbs
      id={database?.id || id}
      name={database?.name || id}
      icon={Database}
      typeLabel={t("databases.resourceType")}
      belongsTo={(ids) => ids.databaseIds}
    />
  );
}

function KeyValueBreadcrumbs({ id }: { id: string }) {
  const { t } = useTranslations();
  const { keyValue } = useKeyValue(id, { poll: false });
  return (
    <DatastoreBreadcrumbs
      id={keyValue?.id || id}
      name={keyValue?.name || id}
      icon={KeyRound}
      typeLabel={t("keyvalue.resourceType")}
      belongsTo={(ids) => ids.keyValueIds}
    />
  );
}

/**
 * Project › Environment › name for a Postgres or Key Value page, the same
 * trail (and switchers) a service gets. A store in no project keeps its type
 * crumb, but the leaf is its name either way, not the raw id (w4/144).
 */
function DatastoreBreadcrumbs({
  id,
  name,
  icon: Icon,
  typeLabel,
  belongsTo,
}: {
  id: string;
  name: string;
  icon: LucideIcon;
  typeLabel: string;
  belongsTo: (owner: {
    databaseIds: string[];
    keyValueIds: string[];
  }) => string[];
}) {
  const { t } = useTranslations();
  const { projects } = useProjects({ poll: false });
  const project = projects.find((item) => belongsTo(item).includes(id));
  const { environments } = useEnvironments(project?.id ?? null, {
    poll: false,
  });
  const environment = environments.find((item) => belongsTo(item).includes(id));

  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      {project ? (
        <>
          <div className="hidden sm:contents">
            <ProjectMenu currentId={project.id} projects={projects} />
            <BreadcrumbSeparator />
          </div>
          {environment ? (
            <>
              <EnvironmentMenu
                projectId={project.id}
                currentId={environment.id}
                environments={environments}
              />
              <BreadcrumbSeparator />
            </>
          ) : null}
        </>
      ) : (
        <>
          <span className="flex items-center gap-1.5 px-2 text-sm font-medium">
            <Icon className="size-4 shrink-0 text-muted-foreground" />
            <span className="hidden truncate sm:inline">{typeLabel}</span>
          </span>
          <BreadcrumbSeparator />
        </>
      )}
      <span className="flex min-w-0 items-center gap-1.5 px-2 text-sm font-medium">
        {project ? (
          <Icon className="size-4 shrink-0 text-muted-foreground" />
        ) : null}
        <span className="max-w-48 truncate">{name}</span>
      </span>
    </nav>
  );
}

/**
 * Project › Environment › name for an environment-scoped group, with the same
 * switchers a service gets; Environment Groups › name for a workspace group
 * (w4/212: the leaf was the raw evg- id, hidden on mobile, with no project).
 * The project comes only from the authorized environment index: while that is
 * unresolved no project is painted, and the leaf waits on the group's own
 * name rather than showing its id.
 */
function EnvGroupBreadcrumbs({ id }: { id: string }) {
  const { t } = useTranslations();
  const { group } = useEnvGroup(id, { poll: false });
  const index = useEnvGroupScopeIndex();
  const environment = group?.environmentId
    ? index.byId.get(group.environmentId)
    : undefined;
  const project = environment
    ? index.projects.find((item) => item.id === environment.projectId)
    : undefined;

  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      {project && environment ? (
        <>
          <div className="hidden sm:contents">
            <ProjectMenu currentId={project.id} projects={index.projects} />
            <BreadcrumbSeparator />
          </div>
          <EnvironmentMenu
            projectId={project.id}
            currentId={environment.id}
            environments={index.environments.filter(
              (item) => item.projectId === project.id,
            )}
          />
          <BreadcrumbSeparator />
        </>
      ) : (
        <>
          <BreadcrumbLink
            to="/env-groups"
            icon={Boxes}
            label={t("common.navEnvGroups")}
          />
          <BreadcrumbSeparator />
        </>
      )}
      <span className="flex min-w-0 items-center gap-1.5 px-2 text-sm font-medium">
        {group ? (
          <span className="max-w-48 truncate">{group.name}</span>
        ) : (
          <Skeleton className="h-4 w-24" />
        )}
      </span>
    </nav>
  );
}

function BuildSourceBreadcrumbs({ id }: { id: string }) {
  const { t } = useTranslations();
  const { buildSource } = useBuildSource(id);
  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      <BreadcrumbLink
        to="/build-sources"
        icon={Package}
        label={t("common.navBuildSources")}
      />
      <BreadcrumbSeparator />
      <span className="flex min-w-0 items-center gap-1.5 px-2 text-sm font-medium">
        <span className="max-w-48 truncate">{buildSource?.name ?? id}</span>
      </span>
    </nav>
  );
}

/**
 * Blueprints › name on a Blueprint's pages, as Render's breadcrumb reads
 * (w3/m677): the name, read with the layout's own query and id, not the raw
 * id; Render's menu of the other Blueprints is not offered, as no bex
 * resource crumb offers one.
 */
function BlueprintBreadcrumbs({ id }: { id: string }) {
  const { t } = useTranslations();
  const { blueprint } = useBlueprint(canonicalBlueprintLinkId(id), {
    poll: false,
  });
  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      <BreadcrumbLink
        to="/blueprints"
        icon={Layers}
        label={t("common.navBlueprints")}
      />
      <BreadcrumbSeparator />
      <span className="flex min-w-0 items-center gap-1.5 px-2 text-sm font-medium">
        {blueprint ? (
          <span className="max-w-48 truncate">{blueprint.name}</span>
        ) : (
          <Skeleton className="h-4 w-24" />
        )}
      </span>
    </nav>
  );
}

function ProjectBreadcrumbs({
  projectId,
  environmentId,
}: {
  projectId: string;
  environmentId?: string;
}) {
  const { t } = useTranslations();
  const { projects } = useProjects({ poll: false });
  const { environments } = useEnvironments(environmentId ? projectId : null, {
    poll: false,
  });
  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      <ProjectMenu currentId={projectId} projects={projects} />
      {environmentId ? (
        // The environment crumb shows from md, as on a service's trail.
        <div className="hidden md:contents">
          <BreadcrumbSeparator />
          <EnvironmentMenu
            projectId={projectId}
            currentId={environmentId}
            environments={environments}
          />
        </div>
      ) : null}
    </nav>
  );
}

function ProjectMenu({
  currentId,
  projects,
}: {
  currentId: string;
  projects: ReturnType<typeof useProjects>["projects"];
}) {
  const { t } = useTranslations();
  const current = projects.find((project) => project.id === currentId);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="min-w-0 max-w-48 shrink gap-1 px-2 font-medium"
        >
          <FolderKanban className="shrink-0" />
          <span className="truncate">{current?.name ?? currentId}</span>
          <ChevronDown className="shrink-0" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuLabel>{t("common.topbarSwitchProject")}</DropdownMenuLabel>
        {projects.map((project) => (
          <DropdownMenuItem key={project.id} asChild>
            <Link
              to="/project/$projectId"
              params={{ projectId: project.id }}
              className="min-w-0"
            >
              <FolderKanban />
              <span className="truncate">{project.name}</span>
            </Link>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/">
            <FolderKanban />
            {t("common.navProjects")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/" search={{ new: "project" }}>
            <Plus />
            {t("projects.createTitle")}
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function EnvironmentMenu({
  projectId,
  currentId,
  environments,
}: {
  projectId: string;
  currentId: string;
  environments: ReturnType<typeof useEnvironments>["environments"];
}) {
  const { t } = useTranslations();
  const current = environments.find(
    (environment) => environment.id === currentId,
  );
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="hidden min-w-0 max-w-40 shrink gap-1 px-2 font-medium md:inline-flex"
        >
          <Boxes className="shrink-0" />
          <span className="truncate">{current?.name ?? currentId}</span>
          <ChevronDown className="shrink-0" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuLabel>
          {t("common.topbarSwitchEnvironment")}
        </DropdownMenuLabel>
        {environments.map((environment) => (
          <DropdownMenuItem key={environment.id} asChild>
            <Link
              to="/project/$projectId/environment/$environmentId"
              params={{ projectId, environmentId: environment.id }}
              className="min-w-0"
            >
              <Boxes />
              <span className="truncate">{environment.name}</span>
            </Link>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function BreadcrumbSeparator() {
  return (
    <ChevronRight
      aria-hidden="true"
      className="hidden size-3.5 shrink-0 text-muted-foreground sm:block"
    />
  );
}

function BreadcrumbLink({
  to,
  icon: Icon,
  label,
}: {
  to: "/" | "/env-groups" | "/build-sources" | "/blueprints";
  icon: LucideIcon;
  label: string;
}) {
  return (
    <Button variant="ghost" size="sm" className="min-w-0 gap-1.5 px-2" asChild>
      {/* The text label is display:none below sm, which drops it from the
          accessible name; name the icon-only link explicitly (w4/254). */}
      <Link to={to} aria-label={label}>
        <Icon className="shrink-0" />
        <span className="hidden truncate sm:inline">{label}</span>
      </Link>
    </Button>
  );
}

type PageDefinition = {
  match: (pathname: string) => boolean;
  labelKey: keyof typeof en;
  icon: LucideIcon;
  /** The section a create page sits under ("Blueprints › New …"). */
  parent?: {
    to: "/blueprints" | "/build-sources";
    labelKey: keyof typeof en;
    icon: LucideIcon;
  };
};

const PAGE_DEFINITIONS: PageDefinition[] = [
  {
    match: (path) => path === "/",
    labelKey: "common.navProjects",
    icon: FolderKanban,
  },
  {
    match: (path) => path.startsWith("/new/workspace"),
    labelKey: "workspaces.newTitle",
    icon: Plus,
  },
  {
    match: (path) => path === "/blueprint/new",
    labelKey: "blueprints.createTitle",
    icon: Plus,
    parent: {
      to: "/blueprints",
      labelKey: "common.navBlueprints",
      icon: Layers,
    },
  },
  {
    // Render's topbar names the page Generate Blueprint opens "Create
    // Blueprint" (w3/m681).
    match: (path) => path === "/blueprint/generate",
    labelKey: "blueprints.generateBreadcrumb",
    icon: Plus,
    parent: {
      to: "/blueprints",
      labelKey: "common.navBlueprints",
      icon: Layers,
    },
  },
  {
    match: (path) => path.startsWith("/blueprints"),
    labelKey: "common.navBlueprints",
    icon: Layers,
  },
  {
    match: (path) => path.startsWith("/env-groups"),
    labelKey: "common.navEnvGroups",
    icon: Boxes,
  },
  {
    match: (path) => path === "/build-sources/new",
    labelKey: "buildSources.createTitle",
    icon: Plus,
    parent: {
      to: "/build-sources",
      labelKey: "common.navBuildSources",
      icon: Package,
    },
  },
  {
    match: (path) => path.startsWith("/build-sources"),
    labelKey: "common.navBuildSources",
    icon: Package,
  },
  {
    match: (path) => path.startsWith("/sandbox-groups"),
    labelKey: "common.navSandboxGroups",
    icon: Box,
  },
  {
    match: (path) => path === "/webhooks/new",
    labelKey: "webhooks.newTitle",
    icon: Plus,
  },
  {
    match: (path) => path.startsWith("/webhook") || path === "/webhooks",
    labelKey: "common.navWebhooks",
    icon: Webhook,
  },
  {
    match: (path) => path === "/router",
    labelKey: "common.navRouter",
    icon: Network,
  },
  {
    match: (path) => path.startsWith("/agents"),
    labelKey: "common.navAgents",
    icon: Bot,
  },
  {
    match: (path) => path.startsWith("/automations"),
    labelKey: "common.navAutomations",
    icon: CalendarClock,
  },
  {
    match: (path) => path.startsWith("/notifications"),
    labelKey: "common.navNotifications",
    icon: Bell,
  },
  {
    match: (path) => path.startsWith("/billing"),
    labelKey: "common.navUsage",
    icon: CreditCard,
  },
  {
    match: (path) => /^\/org\/[^/]+\/workspaces/.test(path),
    labelKey: "organizations.workspacesTitle",
    icon: Building2,
  },
  {
    match: (path) => /^\/org\/[^/]+\/members/.test(path),
    labelKey: "organizations.membersTitle",
    icon: Building2,
  },
  {
    match: (path) => /^\/org\/[^/]+\/settings/.test(path),
    labelKey: "organizations.settingsTitle",
    icon: Building2,
  },
  {
    match: (path) => path.startsWith("/workspace/settings"),
    labelKey: "common.topbarWorkspaceSettings",
    icon: Settings,
  },
  {
    match: (path) => path.startsWith("/settings"),
    labelKey: "common.userMenuSettings",
    icon: UserRound,
  },
  {
    match: (path) => path.startsWith("/databases"),
    labelKey: "databases.resourceType",
    icon: Database,
  },
  {
    match: (path) => path.startsWith("/keyvalue"),
    labelKey: "keyvalue.resourceType",
    icon: KeyRound,
  },
];

/** Detail routes whose breadcrumb trails their resource id (the leaf, in order). */
const DETAIL_ID_PARAMS = [
  "agentSessionId",
  "databaseId",
  "keyValueId",
  "webhookId",
] as const satisfies ReadonlyArray<keyof DashboardParams>;

function PageBreadcrumb({
  pathname,
  params,
}: {
  pathname: string;
  params: DashboardParams;
}) {
  const { t } = useTranslations();
  const definition: Omit<PageDefinition, "match"> = PAGE_DEFINITIONS.find(
    (page) => page.match(pathname),
  ) ?? { labelKey: "common.appName", icon: Globe2 };
  const detailId = DETAIL_ID_PARAMS.map((key) => params[key]).find(Boolean);
  const Icon = definition.icon;

  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-1.5 px-2 text-sm font-medium"
    >
      {definition.parent ? (
        <>
          {/* The label is display:none below sm, which drops it from the
              accessible name; name the icon-only link explicitly (w4/254). */}
          <Link
            to={definition.parent.to}
            aria-label={t(definition.parent.labelKey)}
            className="text-muted-foreground hover:text-foreground flex shrink-0 items-center gap-1.5"
          >
            <definition.parent.icon className="size-4 shrink-0" />
            <span className="hidden truncate sm:inline">
              {t(definition.parent.labelKey)}
            </span>
          </Link>
          <BreadcrumbSeparator />
          <span className="max-w-56 truncate">{t(definition.labelKey)}</span>
        </>
      ) : (
        <>
          <Icon className="size-4 shrink-0 text-muted-foreground" />
          <span className="max-w-48 truncate">{t(definition.labelKey)}</span>
        </>
      )}
      {detailId ? (
        <>
          <BreadcrumbSeparator />
          <span className="hidden max-w-40 truncate text-muted-foreground sm:inline">
            {detailId}
          </span>
        </>
      ) : null}
    </nav>
  );
}
