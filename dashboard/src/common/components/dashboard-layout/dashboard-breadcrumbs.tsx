import { Link, useParams, useRouterState } from "@tanstack/react-router";
import {
  BarChart3,
  Bell,
  Boxes,
  ChevronDown,
  ChevronRight,
  Database,
  FolderKanban,
  Globe2,
  KeyRound,
  Layers,
  Plus,
  Settings,
  UserRound,
  Webhook,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
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
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useServer } from "@/features/services/hooks/use-server";
import { useServices } from "@/features/services/hooks/use-services";

type DashboardParams = {
  serviceId?: string;
  projectId?: string;
  databaseId?: string;
  keyValueId?: string;
  groupId?: string;
  blueprintId?: string;
  webhookId?: string;
};

/** Contextual hierarchy shown on the left of every dashboard topbar. */
export function DashboardBreadcrumbs() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const params = useParams({ strict: false }) as DashboardParams;

  if (params.serviceId) {
    return <ServiceBreadcrumbs serviceId={params.serviceId} />;
  }
  if (params.projectId) {
    return <ProjectBreadcrumbs projectId={params.projectId} />;
  }
  return <PageBreadcrumb pathname={pathname} params={params} />;
}

function ServiceBreadcrumbs({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { service } = useServer(serviceId);
  const { services } = useServices();
  const { projects } = useProjects();
  const resolvedServiceId = service?.id ?? serviceId;
  const project = projects.find((item) =>
    item.serviceIds.includes(resolvedServiceId),
  );
  const { environments } = useEnvironments(project?.id ?? null);
  const environment = environments.find((item) =>
    item.serviceIds.includes(resolvedServiceId),
  );

  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-0.5"
    >
      <div className="hidden sm:contents">
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
      </div>
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
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="min-w-0 max-w-48 gap-1 px-2 font-medium"
          >
            <Globe2 className="shrink-0" />
            <span className="truncate">{service?.name ?? serviceId}</span>
            <ChevronDown className="shrink-0" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-64">
          <DropdownMenuLabel>
            {t("common.topbarSwitchService")}
          </DropdownMenuLabel>
          {services.map((item) => (
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

function ProjectBreadcrumbs({ projectId }: { projectId: string }) {
  const { t } = useTranslations();
  const { projects } = useProjects();
  return (
    <nav aria-label={t("common.topbarBreadcrumbs")} className="min-w-0">
      <ProjectMenu currentId={projectId} projects={projects} />
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
          className="min-w-0 max-w-48 gap-1 px-2 font-medium"
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
          className="hidden min-w-0 max-w-40 gap-1 px-2 font-medium md:inline-flex"
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
              to="/project/$projectId"
              params={{ projectId }}
              search={{ env: environment.id }}
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
  to: "/";
  icon: LucideIcon;
  label: string;
}) {
  return (
    <Button variant="ghost" size="sm" className="min-w-0 gap-1.5 px-2" asChild>
      <Link to={to}>
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
};

const PAGE_DEFINITIONS: PageDefinition[] = [
  {
    match: (path) => path === "/",
    labelKey: "common.navProjects",
    icon: FolderKanban,
  },
  {
    match: (path) => path.startsWith("/services/new"),
    labelKey: "services.createTitle",
    icon: Plus,
  },
  {
    match: (path) => path.startsWith("/keyvalue/new"),
    labelKey: "keyvalue.createTitle",
    icon: Plus,
  },
  {
    match: (path) => path.startsWith("/new/workspace"),
    labelKey: "workspaces.newTitle",
    icon: Plus,
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
    match: (path) => path.startsWith("/notifications"),
    labelKey: "common.navNotifications",
    icon: Bell,
  },
  {
    match: (path) => path.startsWith("/usage"),
    labelKey: "common.navUsage",
    icon: BarChart3,
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

function PageBreadcrumb({
  pathname,
  params,
}: {
  pathname: string;
  params: DashboardParams;
}) {
  const { t } = useTranslations();
  const definition = PAGE_DEFINITIONS.find((page) => page.match(pathname)) ?? {
    labelKey: "common.appName" as const,
    icon: Globe2,
  };
  const detailId =
    params.databaseId ??
    params.keyValueId ??
    params.groupId ??
    params.blueprintId ??
    params.webhookId;
  const Icon = definition.icon;

  return (
    <nav
      aria-label={t("common.topbarBreadcrumbs")}
      className="flex min-w-0 items-center gap-1.5 px-2 text-sm font-medium"
    >
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <span className="max-w-48 truncate">{t(definition.labelKey)}</span>
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
