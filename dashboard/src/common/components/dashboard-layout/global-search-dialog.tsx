import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  Bell,
  Boxes,
  CreditCard,
  Database,
  FolderKanban,
  KeyRound,
  Layers,
  Settings,
  UserRound,
  Webhook,
} from "lucide-react";
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
import {
  deriveServiceType,
  SERVICE_TYPE_ICON,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";

/**
 * cmdk-backed dialog body — kept in its own module so the persistent header
 * only pays for it after the user opens search (bundle-conditional).
 */
export function GlobalSearchDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");

  return (
    <CommandDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setSearch("");
        onOpenChange(next);
      }}
      shouldFilter={false}
    >
      <DialogTitle className="sr-only">{t("common.topbarSearch")}</DialogTitle>
      <DialogDescription className="sr-only">
        {t("common.topbarSearchDescription")}
      </DialogDescription>
      <CommandInput
        placeholder={t("common.topbarSearchPlaceholder")}
        value={search}
        onValueChange={setSearch}
      />
      <CommandList>
        <CommandEmpty>{t("common.topbarSearchEmpty")}</CommandEmpty>
        {open ? (
          <SearchResults search={search} close={() => onOpenChange(false)} />
        ) : null}
      </CommandList>
    </CommandDialog>
  );
}

// cmdk's bundled fuzzy scorer treats a subsequence match against a resource's
// ~20-char near-random id as relevant, so a short query like "db" or "cms"
// used to return most of the workspace. shouldFilter={false} above hands
// filtering to this literal, case-insensitive substring match instead —
// mirrors combobox.tsx's precedent — while still matching against the same
// composite name/id/type text so pasting a raw id fragment still finds it.
function matchesQuery(haystack: string, query: string): boolean {
  return query === "" || haystack.toLowerCase().includes(query);
}

function SearchResults({
  search,
  close,
}: {
  search: string;
  close: () => void;
}) {
  const { t } = useTranslations();
  const query = search.trim().toLowerCase();
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
      icon: CreditCard,
      run: () => void navigate({ to: "/billing" }),
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

  const filteredPages = pages.filter((page) => matchesQuery(page.label, query));
  const filteredProjects = projects.filter((r) =>
    matchesQuery(
      `${r.name} ${r.id} ${t("common.topbarProjectResource")}`,
      query,
    ),
  );
  const filteredServices = services.filter((r) =>
    matchesQuery(
      `${r.name} ${r.id} ${t("common.topbarServiceResource")}`,
      query,
    ),
  );
  const filteredDatabases = databases.filter((r) =>
    matchesQuery(`${r.name} ${r.id} ${t("databases.resourceType")}`, query),
  );
  const filteredKeyValues = keyValues.filter((r) =>
    matchesQuery(`${r.name} ${r.id} ${t("keyvalue.resourceType")}`, query),
  );
  const filteredEnvGroups = envGroups.filter((r) =>
    matchesQuery(`${r.name} ${r.id} ${t("envGroups.resourceType")}`, query),
  );

  // shouldFilter={false} means cmdk never prunes an itemless group for us — an
  // unguarded group would float its heading over zero children (w6/046). Render
  // each group only when it has content, and the separator only between two.
  const showNavigation = filteredPages.length > 0;
  const showResources =
    filteredProjects.length > 0 ||
    filteredServices.length > 0 ||
    filteredDatabases.length > 0 ||
    filteredKeyValues.length > 0 ||
    filteredEnvGroups.length > 0 ||
    (loading && resourceCount === 0);

  return (
    <>
      {showNavigation ? (
        <CommandGroup heading={t("common.topbarNavigation")}>
          {filteredPages.map((page) => (
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
      ) : null}
      {showNavigation && showResources ? <CommandSeparator /> : null}
      {showResources ? (
        <CommandGroup heading={t("common.topbarResources")}>
          {loading && resourceCount === 0 ? (
            <CommandItem disabled>{t("common.loading")}</CommandItem>
          ) : null}
          {filteredProjects.map((project) => (
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
          {filteredServices.map((service) => {
            // Per-type label + icon from the shared service-type helpers — the
            // same ones the detail header and resource table use — so the
            // palette names a Cron Job / Private Service by its kind instead of
            // collapsing every service to a generic "Service" + Globe (w4/049).
            const typeKey = deriveServiceType(service.type);
            const TypeIcon = SERVICE_TYPE_ICON[typeKey];
            const typeLabel = t(SERVICE_TYPE_LABEL[typeKey]);
            return (
              <CommandItem
                key={`service:${service.id}`}
                // Keep the generic token so typing "service" still matches every
                // service, and add the specific words so "cron"/"private" match too.
                value={`${service.name} ${service.id} ${t("common.topbarServiceResource")} ${typeLabel}`}
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
                <TypeIcon />
                <SearchResultLabel name={service.name} kind={typeLabel} />
              </CommandItem>
            );
          })}
          {filteredDatabases.map((database) => (
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
          {filteredKeyValues.map((keyValue) => (
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
          {filteredEnvGroups.map((group) => (
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
      ) : null}
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
