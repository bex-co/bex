import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  BarChart3,
  Bell,
  Boxes,
  Database,
  FolderKanban,
  Globe2,
  KeyRound,
  Layers,
  Search,
  Settings,
  UserRound,
  Webhook,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/common/components/ui/command";
import { DialogDescription, DialogTitle } from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useServices } from "@/features/services/hooks/use-services";
import { serviceBaseForType } from "@/features/services/lib/service-base";

/**
 * Workspace-wide command search, opened from any dashboard page or with
 * Cmd/Ctrl+K. Resource hooks mount only while the dialog is open, keeping the
 * persistent topbar free of additional page-load queries.
 */
export function GlobalSearch() {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen((value) => !value);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className="gap-2 px-2 text-muted-foreground hover:text-foreground sm:px-3"
        onClick={() => setOpen(true)}
        aria-label={t("common.topbarSearch")}
      >
        <Search />
        <span className="hidden md:inline">{t("common.topbarSearch")}</span>
        <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 font-sans text-[10px] font-medium lg:inline">
          {typeof navigator !== "undefined" &&
          /Mac|iPhone|iPad/.test(navigator.platform)
            ? "⌘ K"
            : "Ctrl K"}
        </kbd>
      </Button>
      <CommandDialog open={open} onOpenChange={setOpen}>
        <DialogTitle className="sr-only">
          {t("common.topbarSearch")}
        </DialogTitle>
        <DialogDescription className="sr-only">
          {t("common.topbarSearchDescription")}
        </DialogDescription>
        <CommandInput placeholder={t("common.topbarSearchPlaceholder")} />
        <CommandList>
          <CommandEmpty>{t("common.topbarSearchEmpty")}</CommandEmpty>
          {open ? <SearchResults close={() => setOpen(false)} /> : null}
        </CommandList>
      </CommandDialog>
    </>
  );
}

function SearchResults({ close }: { close: () => void }) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { services, loading: servicesLoading } = useServices();
  const { databases, loading: databasesLoading } = useDatabases();
  const { keyValues, loading: keyValuesLoading } = useKeyValues();
  const { projects, loading: projectsLoading } = useProjects();
  const { groups: envGroups, loading: envGroupsLoading } = useEnvGroups();

  function select(run: () => void) {
    close();
    run();
  }

  const pages = [
    {
      label: t("common.navProjects"),
      icon: FolderKanban,
      run: () => void navigate({ to: "/" }),
    },
    {
      label: t("common.navBlueprints"),
      icon: Layers,
      run: () => void navigate({ to: "/blueprints" }),
    },
    {
      label: t("common.navEnvGroups"),
      icon: Boxes,
      run: () => void navigate({ to: "/env-groups" }),
    },
    {
      label: t("common.navWebhooks"),
      icon: Webhook,
      run: () => void navigate({ to: "/webhooks" }),
    },
    {
      label: t("common.navNotifications"),
      icon: Bell,
      run: () => void navigate({ to: "/notifications" }),
    },
    {
      label: t("common.navUsage"),
      icon: BarChart3,
      run: () => void navigate({ to: "/usage" }),
    },
    {
      label: t("common.topbarWorkspaceSettings"),
      icon: Settings,
      run: () => void navigate({ to: "/workspace/settings" }),
    },
    {
      label: t("common.userMenuSettings"),
      icon: UserRound,
      run: () => void navigate({ to: "/settings" }),
    },
  ];

  const loading =
    servicesLoading ||
    databasesLoading ||
    keyValuesLoading ||
    projectsLoading ||
    envGroupsLoading;
  const resourceCount =
    services.length +
    databases.length +
    keyValues.length +
    projects.length +
    envGroups.length;

  return (
    <>
      <CommandGroup heading={t("common.topbarNavigation")}>
        {pages.map((page) => (
          <CommandItem
            key={page.label}
            value={page.label}
            onSelect={() => select(page.run)}
          >
            <page.icon />
            {page.label}
          </CommandItem>
        ))}
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup heading={t("common.topbarResources")}>
        {loading && resourceCount === 0 ? (
          <CommandItem disabled>{t("common.loading")}</CommandItem>
        ) : null}
        {projects.map((project) => (
          <CommandItem
            key={`project:${project.id}`}
            value={`${project.name} ${project.id} ${t("common.topbarProjectResource")}`}
            onSelect={() =>
              select(() => {
                void navigate({
                  to: "/project/$projectId",
                  params: { projectId: project.id },
                });
              })
            }
          >
            <FolderKanban />
            <SearchResultLabel
              name={project.name}
              kind={t("common.topbarProjectResource")}
            />
          </CommandItem>
        ))}
        {services.map((service) => (
          <CommandItem
            key={`service:${service.id}`}
            value={`${service.name} ${service.id} ${t("common.topbarServiceResource")}`}
            onSelect={() =>
              select(() => {
                // Canonical base per type — routing a static_site through
                // /services/<id> costs an extra bounce navigation.
                if (serviceBaseForType(service.type) === "/static") {
                  void navigate({
                    to: "/static/$serviceId",
                    params: { serviceId: service.id },
                  });
                } else {
                  void navigate({
                    to: "/services/$serviceId",
                    params: { serviceId: service.id },
                  });
                }
              })
            }
          >
            <Globe2 />
            <SearchResultLabel
              name={service.name}
              kind={t("common.topbarServiceResource")}
            />
          </CommandItem>
        ))}
        {databases.map((database) => (
          <CommandItem
            key={`database:${database.id}`}
            value={`${database.name} ${database.id} ${t("databases.resourceType")}`}
            onSelect={() =>
              select(() => {
                void navigate({
                  to: "/databases/$databaseId",
                  params: { databaseId: database.id },
                });
              })
            }
          >
            <Database />
            <SearchResultLabel
              name={database.name}
              kind={t("databases.resourceType")}
            />
          </CommandItem>
        ))}
        {keyValues.map((keyValue) => (
          <CommandItem
            key={`keyvalue:${keyValue.id}`}
            value={`${keyValue.name} ${keyValue.id} ${t("keyvalue.resourceType")}`}
            onSelect={() =>
              select(() => {
                void navigate({
                  to: "/keyvalue/$keyValueId",
                  params: { keyValueId: keyValue.id },
                });
              })
            }
          >
            <KeyRound />
            <SearchResultLabel
              name={keyValue.name}
              kind={t("keyvalue.resourceType")}
            />
          </CommandItem>
        ))}
        {envGroups.map((group) => (
          <CommandItem
            key={`env-group:${group.id}`}
            value={`${group.name} ${group.id} ${t("envGroups.resourceType")}`}
            onSelect={() =>
              select(() => {
                void navigate({
                  to: "/env-groups/$groupId",
                  params: { groupId: group.id },
                });
              })
            }
          >
            <Boxes />
            <SearchResultLabel
              name={group.name}
              kind={t("envGroups.resourceType")}
            />
          </CommandItem>
        ))}
      </CommandGroup>
    </>
  );
}

function SearchResultLabel({ name, kind }: { name: string; kind: string }) {
  return (
    <span className="flex min-w-0 flex-1 items-center justify-between gap-3">
      <span className="truncate">{name}</span>
      <span className="shrink-0 text-xs text-muted-foreground">{kind}</span>
    </span>
  );
}
