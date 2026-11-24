import { useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  Boxes,
  Database,
  FolderKanban,
  KeyRound,
  Layers,
  Package,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import {
  CommandDialog,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/common/components/ui/command";
import { DialogDescription, DialogTitle } from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentsFeatureEnabled } from "@/config/use-growthbook";
import { NAV_GROUPS, visibleNavGroups } from "./dashboard-nav";
import {
  matchesShortcut,
  PAGE_KEYWORDS,
  SEARCH_SHORTCUTS,
} from "./search-shortcuts";
import { useRouterNavAvailable } from "./use-router-nav-available";
import { useWorkspaceAdmin } from "@/features/api-keys/hooks/use-workspace-admin";
import { useBlueprints } from "@/features/blueprints/hooks/use-blueprints";
import { useBuildSourcesList } from "@/features/build-sources/hooks/use-build-sources-list";
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
  returnFocus,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /**
   * Puts focus back where it was before the search opened. Called when the
   * dialog is dismissed, not when a result is chosen: that navigates, and the
   * destination must keep its own focus (w4/291).
   */
  returnFocus?: () => void;
}) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");
  // Reset on the prop's false→true edge, during render (w4/265): result
  // selection and the parent's Cmd/Ctrl+K toggle close without onOpenChange,
  // and an effect would paint the old query for a frame on reopen.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setSearch("");
  }
  // Set by a result's selection so the close that follows is not a dismissal.
  const choseResult = useRef(false);

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      shouldFilter={false}
      label={t("common.topbarSearch")}
      onCloseAutoFocus={(event) => {
        // The Search button is not this dialog's Radix trigger, so the default
        // restores focus to nothing and leaves it on <body>.
        event.preventDefault();
        if (choseResult.current) {
          choseResult.current = false;
          return;
        }
        returnFocus?.();
      }}
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
        {open ? (
          <SearchResults
            search={search}
            close={() => {
              choseResult.current = true;
              onOpenChange(false);
            }}
          />
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
  // Blueprints are a first-class sidebar resource (w4/147). Webhooks stay out
  // on purpose: an endpoint is named by its URL, a workspace has a handful, and
  // the Webhooks page is one Navigation result away.
  const { blueprints, loading: blueprintsLoading } = useBlueprints();
  const { buildSources, loading: buildSourcesLoading } = useBuildSourcesList();

  function select(run: () => void) {
    close();
    run();
  }

  // The palette's pages are the sidebar's own destinations behind the same
  // feature gates (w21/031); the sidebar's one Settings entry becomes the
  // palette's two, Workspace and Account.
  const agentsEnabled = useAgentsFeatureEnabled();
  const routerAvailable = useRouterNavAvailable();
  const pages = visibleNavGroups(NAV_GROUPS, { agentsEnabled, routerAvailable })
    .flatMap((group) => group.items)
    .flatMap((item) => {
      const page = (
        label: string,
        icon: LucideIcon | undefined,
        to: string,
      ) => ({
        label,
        icon,
        keywords: PAGE_KEYWORDS[to] ?? [],
        run: () => void navigate({ to }),
      });
      if (item.to === "/workspace/settings") {
        return [
          page(t("common.topbarWorkspaceSettings"), item.icon, item.to),
          page(t("common.userMenuSettings"), UserRound, "/settings"),
        ];
      }
      return [page(t(item.labelKey), item.icon, item.to)];
    });
  // Render's search shortcuts to a Settings section or a docs page (w4/711).
  // Like Render's, they answer only a query of three or more characters, so
  // the open palette still lists just the pages. Audit Logs is only on an
  // Admin's Workspace Settings, so only an Admin is offered it.
  const admin = useWorkspaceAdmin() === true;
  const shortcuts = SEARCH_SHORTCUTS.filter(
    (shortcut) => admin || !shortcut.adminOnly,
  ).map((shortcut) => ({
    id: shortcut.id,
    label: t(shortcut.labelKey),
    description: t(shortcut.descriptionKey),
    keywords: shortcut.keywords,
    icon: shortcut.icon,
    run:
      "href" in shortcut
        ? () => void window.open(shortcut.href, "_blank", "noopener,noreferrer")
        : () => void navigate({ to: shortcut.to, hash: shortcut.hash }),
  }));

  const loading =
    servicesLoading ||
    databasesLoading ||
    keyValuesLoading ||
    projectsLoading ||
    envGroupsLoading ||
    blueprintsLoading ||
    buildSourcesLoading;

  const filteredPages = pages.filter(
    (page) =>
      matchesQuery(page.label, query) ||
      matchesShortcut(query, page.label, page.keywords),
  );
  const filteredShortcuts = shortcuts.filter((shortcut) =>
    matchesShortcut(query, shortcut.label, shortcut.keywords),
  );
  const filteredProjects = projects.filter((r) =>
    matchesQuery(
      `${r.name} ${r.id} ${t("common.topbarProjectResource")}`,
      query,
    ),
  );
  // What a service query matches: the generic "Service" token plus the kind
  // label the row displays, so "cron" or "Cron Job" finds every cron job and a
  // zh user's kind word matches too (w4/342). cmdk never filters on an item's
  // value here (shouldFilter={false}), so only this filter decides; the value
  // reuses the same text as the item's identity.
  const serviceHaystack = (service: (typeof services)[number]) =>
    `${serviceSearchText(service)} ${t("common.topbarServiceResource")} ${t(SERVICE_TYPE_LABEL[deriveServiceType(service.type)])}`;
  const filteredServices = services.filter((r) =>
    matchesQuery(serviceHaystack(r), query),
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
  const filteredBlueprints = blueprints.filter((r) =>
    matchesQuery(`${r.name} ${r.id} ${t("blueprints.resourceType")}`, query),
  );
  const buildSourceHaystack = (source: (typeof buildSources)[number]) =>
    [
      source.name,
      source.id,
      source.git?.repoUrl,
      source.git?.branch,
      source.image?.imageUrl,
      t("buildSources.resourceType"),
    ]
      .filter(Boolean)
      .join(" ");
  const filteredBuildSources = buildSources.filter((r) =>
    matchesQuery(buildSourceHaystack(r), query),
  );

  // shouldFilter={false} means cmdk never prunes an itemless group for us — an
  // unguarded group would float its heading over zero children (w6/046). Render
  // each group only when it has content, and the separator only between two.
  // While any list is still on its first read the Resources group keeps a
  // Loading row, even beside unrelated cached rows: the match may be in the
  // list that hasn't arrived (w4/229).
  const showNavigation =
    filteredPages.length > 0 || filteredShortcuts.length > 0;
  const showResources =
    filteredProjects.length > 0 ||
    filteredServices.length > 0 ||
    filteredDatabases.length > 0 ||
    filteredKeyValues.length > 0 ||
    filteredEnvGroups.length > 0 ||
    filteredBlueprints.length > 0 ||
    filteredBuildSources.length > 0 ||
    loading;

  // "No matches" is decided here, from settled lists and the computed matches —
  // not by cmdk's Command.Empty, which only counts currently registered items
  // and so declared a definitive empty while lists were still loading (w4/229).
  if (!showNavigation && !showResources) {
    return (
      <div role="status" className="py-6 text-center text-sm">
        {t("common.topbarSearchEmpty")}
      </div>
    );
  }

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
              {page.icon ? <page.icon /> : null}
              {page.label}
            </CommandItem>
          ))}
          {filteredShortcuts.map((shortcut) => (
            <CommandItem
              key={`shortcut:${shortcut.id}`}
              value={`shortcut:${shortcut.id}`}
              onSelect={() => select(shortcut.run)}
            >
              <shortcut.icon />
              <ShortcutLabel
                name={shortcut.label}
                description={shortcut.description}
              />
            </CommandItem>
          ))}
        </CommandGroup>
      ) : null}
      {showNavigation && showResources ? <CommandSeparator /> : null}
      {showResources ? (
        <CommandGroup heading={t("common.topbarResources")}>
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
                value={serviceHaystack(service)}
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
                <SearchResultLabel
                  name={service.name}
                  // A renamed service still answers to its original name (its
                  // host and Blueprint `name:`), so show why it matched.
                  aka={originalName(service)}
                  kind={typeLabel}
                />
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
          {filteredBlueprints.map((blueprint) => (
            <CommandItem
              key={`blueprint:${blueprint.id}`}
              value={`${blueprint.name} ${blueprint.id} ${t("blueprints.resourceType")}`}
              onSelect={() =>
                select(() => {
                  void navigate({
                    to: "/blueprints/$blueprintId",
                    params: { blueprintId: blueprint.id },
                  });
                })
              }
            >
              <Layers />
              <SearchResultLabel
                name={blueprint.name}
                kind={t("blueprints.resourceType")}
              />
            </CommandItem>
          ))}
          {filteredBuildSources.map((source) => (
            <CommandItem
              key={`build-source:${source.id}`}
              value={buildSourceHaystack(source)}
              onSelect={() =>
                select(() => {
                  void navigate({
                    to: "/build-sources",
                    search: { q: source.name, kind: undefined },
                  });
                })
              }
            >
              <Package />
              <SearchResultLabel
                name={source.name}
                kind={t("buildSources.resourceType")}
              />
            </CommandItem>
          ))}
          {/* Last, so already-matching rows above it never shift when it
              leaves; disabled, so cmdk's keyboard selection skips it. */}
          {loading ? (
            <CommandItem disabled>{t("common.loading")}</CommandItem>
          ) : null}
        </CommandGroup>
      ) : null}
    </>
  );
}

/** Every name a service answers to: its display name, the immutable name and
 *  slug a rename keeps (its host, a Blueprint's `name:`), and its id (w4/147). */
function serviceSearchText(service: {
  name: string;
  id: string;
  immutableName?: string | null;
  slug: string | null;
}): string {
  return [service.name, service.immutableName, service.slug, service.id]
    .filter(Boolean)
    .join(" ");
}

/** The original name to show beside a renamed service, or undefined. */
function originalName(service: {
  name: string;
  immutableName?: string | null;
}): string | undefined {
  const original = service.immutableName ?? "";
  return original && original !== service.name ? original : undefined;
}

/** A shortcut's name, and Render's one-line description of where it leads,
 *  which gives way first on a narrow palette. */
function ShortcutLabel({
  name,
  description,
}: {
  name: string;
  description: string;
}) {
  return (
    <span className="flex min-w-0 flex-1 items-center justify-between gap-3">
      <span className="shrink-0">{name}</span>
      {/* A real space keeps the two apart in the accessible name (w21/018). */}{" "}
      <span
        title={description}
        className="min-w-0 truncate text-xs text-muted-foreground"
      >
        {description}
      </span>
    </span>
  );
}

function SearchResultLabel({
  name,
  aka,
  kind,
}: {
  name: string;
  aka?: string;
  kind: string;
}) {
  return (
    <span className="flex min-w-0 flex-1 items-center justify-between gap-3">
      <span className="truncate">
        {name}
        {aka ? (
          <>
            {/* A real space: a margin alone glues the two names in the
                accessible name (w21/018). */}{" "}
            <span className="ml-1 text-xs text-muted-foreground">{aka}</span>
          </>
        ) : null}
      </span>
      <span className="shrink-0 text-xs text-muted-foreground">{kind}</span>
    </span>
  );
}
