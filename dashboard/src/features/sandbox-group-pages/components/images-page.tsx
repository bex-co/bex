import { useState } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/common/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { LocalDateTime } from "@/common/components/local-time";
import { useDebounce } from "@/common/hooks/use-debounce";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useCursorPages,
  useGroupImageSources,
  useImageVersions,
  useSandboxesStartedFrom,
  type ImageSourceRow,
} from "@/features/sandbox-group-pages/hooks/use-group-pages";
import { useSandboxGroupDetail } from "@/features/sandbox-group-pages/lib/group-detail";
import {
  imageFailureKey,
  imageSourceStatus,
  isBuildInProgress,
  shortDigest,
} from "@/features/sandbox-group-pages/lib/sandbox-view";
import {
  cliCreateSandbox,
  cliImportImage,
  IMAGE_SDK_FILE,
  sdkImportImage,
} from "@/features/sandbox-group-pages/lib/starter-code";
import {
  BuildStatusBadge,
  DetailRow,
  LoadError,
  PagingControls,
  SearchBox,
  StarterPanel,
  StartedSandboxesList,
} from "./parts";
import { DrawerBodySkeleton, ImagesListSkeleton } from "./group-page-skeletons";
import {
  IMAGE_DRAWER_TABS,
  type ImageDrawerTab,
} from "@/features/sandbox-group-pages/lib/drawer-tabs";

const TAB_LABEL = {
  versions: "sandboxGroupPages.tabVersions",
  details: "sandboxGroupPages.tabDetails",
  sandboxes: "sandboxGroupPages.navSandboxes",
} as const;

/** A failure reason in Render's words where they have them. */
function FailureReason({ reason }: { reason: string | null }) {
  const { t } = useTranslations();
  if (!reason) return null;
  const known = imageFailureKey(reason);
  return (
    <span className="text-muted-foreground block text-xs">
      {known ? t(known.key, known.params) : reason}
    </span>
  );
}

function sourceRef(source: {
  repository: string;
  image: string;
  reference: string;
}) {
  const sep = source.reference.startsWith("sha256:") ? "@" : ":";
  return `${source.repository}/${source.image}${sep}${source.reference}`;
}

/**
 * A sandbox group's Custom images page, Render's (w1/m658): one row per
 * image name with its status, latest version and latest available one, and
 * a drawer with the name's versions, its details and the sandboxes started
 * from it. Images are imported with the CLI or an SDK; the page imports and
 * deletes nothing, as Render's does not.
 */
export function ImagesPage({
  openImage,
  openTab,
  onOpen,
  onClose,
}: {
  openImage: string | undefined;
  openTab: ImageDrawerTab;
  onOpen: (name: string, tab?: ImageDrawerTab) => void;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const [search, setSearch] = useState("");
  const pages = useCursorPages();
  const query = useDebounce(search);
  const list = useGroupImageSources(
    group.id,
    group.ownerId,
    query,
    pages.cursor,
  );
  // A new search starts again at the first page.
  const onSearch = (value: string) => {
    setSearch(value);
    pages.reset();
  };
  const empty = !list.loading && !list.error && list.sources.length === 0;

  return (
    <div className="space-y-6">
      <div data-skeleton-region="images-toolbar" className="flex">
        <SearchBox
          value={search}
          onChange={onSearch}
          placeholder={t("sandboxGroupPages.searchImages")}
        />
      </div>
      {list.loading ? (
        <ImagesListSkeleton />
      ) : list.error ? (
        <LoadError
          title={t("sandboxGroupPages.loadImagesError")}
          onRetry={() => void list.refetch()}
        />
      ) : empty && search.trim() !== "" ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border py-10 text-center">
          <p className="text-sm font-medium">
            {t("sandboxGroupPages.noMatchingImages")}
          </p>
          <Button size="sm" variant="outline" onClick={() => onSearch("")}>
            {t("sandboxGroupPages.clearFilters")}
          </Button>
        </div>
      ) : empty ? (
        <StarterPanel
          title={t("sandboxGroupPages.firstImageTitle")}
          description={t("sandboxGroupPages.firstImageDescription")}
          workspaceId={group.ownerId}
          cli={[
            {
              title: t("sandboxGroupPages.starterImportImage"),
              command: cliImportImage(group.id),
            },
            {
              title: t("sandboxGroupPages.starterStartFromImage"),
              command: cliCreateSandbox(group.id, "agent-runtime"),
            },
          ]}
          sdk={{
            title: t("sandboxGroupPages.starterImportCode"),
            file: IMAGE_SDK_FILE,
            code: (language, apiBaseUrl) =>
              sdkImportImage(language, {
                apiBaseUrl,
                workspaceId: group.ownerId,
                sandboxGroupId: group.id,
              }),
          }}
        />
      ) : (
        <ImageTable rows={list.sources} onOpen={(name) => onOpen(name)} />
      )}
      <PagingControls
        page={pages.page}
        hasMore={list.hasMore}
        onPrevious={pages.previous}
        onNext={() => pages.next(list.cursor)}
      />
      {openImage ? (
        <ImageDrawer
          name={openImage}
          tab={openTab}
          source={list.sources.find((s) => s.name === openImage) ?? null}
          onTabChange={(tab) => onOpen(openImage, tab)}
          onClose={onClose}
        />
      ) : null}
    </div>
  );
}

function ImageStatus({ row }: { row: ImageSourceRow }) {
  const { t } = useTranslations();
  const key = imageSourceStatus(row);
  const variant =
    key === "sandboxGroupPages.imageAvailable"
      ? "success"
      : key === "sandboxGroupPages.imageUnavailable"
        ? "destructive"
        : "secondary";
  return <Badge variant={variant}>{t(key)}</Badge>;
}

function ImageTable({
  rows,
  onOpen,
}: {
  rows: ImageSourceRow[];
  onOpen: (name: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <>
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("sandboxGroupPages.colImage")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colStatus")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colSourceImage")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colLatestAvailable")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colUpdated")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={row.name}
                className="cursor-pointer"
                onClick={() => onOpen(row.name)}
              >
                <TableCell className="max-w-56">
                  <button
                    type="button"
                    className="block max-w-full truncate text-left font-medium hover:underline"
                    onClick={(e) => {
                      e.stopPropagation();
                      onOpen(row.name);
                    }}
                  >
                    {row.name}
                  </button>
                </TableCell>
                <TableCell>
                  <ImageStatus row={row} />
                  {row.latestBuild.status === "failed" ? (
                    <FailureReason reason={row.latestBuild.failureReason} />
                  ) : null}
                </TableCell>
                <TableCell className="max-w-64 truncate font-mono text-xs">
                  {sourceRef(row.source)}
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {row.latestSucceededBuild
                    ? shortDigest(row.latestSucceededBuild.sourceDigest)
                    : "—"}
                </TableCell>
                <TableCell className="text-muted-foreground text-sm">
                  <LocalDateTime value={row.latestBuild.createdAt} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <ul className="space-y-2 md:hidden">
        {rows.map((row) => (
          <li key={row.name}>
            <button
              type="button"
              className="w-full space-y-1 rounded-lg border p-3 text-left"
              onClick={() => onOpen(row.name)}
            >
              <span className="flex items-center justify-between gap-2">
                <span className="truncate text-sm font-medium">{row.name}</span>
                <ImageStatus row={row} />
              </span>
              <span className="text-muted-foreground block truncate font-mono text-xs">
                {sourceRef(row.source)}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}

function ImageDrawer({
  name,
  tab,
  source,
  onTabChange,
  onClose,
}: {
  name: string;
  tab: ImageDrawerTab;
  source: ImageSourceRow | null;
  onTabChange: (tab: ImageDrawerTab) => void;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const { versions, loading, error, refetch } = useImageVersions(
    group.id,
    group.ownerId,
    name,
  );
  const latestAvailable =
    versions.find((v) => v.status === "succeeded") ?? null;
  // Read only for its own tab.
  const started = useSandboxesStartedFrom(
    group.id,
    group.ownerId,
    tab === "sandboxes" && versions.length > 0
      ? { buildIds: versions.map((v) => v.id) }
      : null,
  );
  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent
        side="right"
        className="w-full gap-0 overflow-y-auto sm:max-w-xl"
      >
        {loading ? (
          <DrawerBodySkeleton />
        ) : error ? (
          <div className="p-4 pr-12">
            <SheetTitle className="sr-only">{name}</SheetTitle>
            <LoadError
              title={t("sandboxGroupPages.loadImagesError")}
              onRetry={() => void refetch()}
            />
          </div>
        ) : versions.length === 0 ? (
          <SheetHeader>
            <SheetTitle>{t("sandboxGroupPages.imageGoneTitle")}</SheetTitle>
            <SheetDescription>
              {t("sandboxGroupPages.imageGoneDescription", { name })}
            </SheetDescription>
          </SheetHeader>
        ) : (
          <>
            <SheetHeader className="border-b pr-12">
              <div className="flex items-center gap-2">
                <SheetTitle className="truncate">{name}</SheetTitle>
                {source ? <ImageStatus row={source} /> : null}
              </div>
              <SheetDescription className="font-mono text-xs">
                {sourceRef(versions[0].source)}
              </SheetDescription>
            </SheetHeader>
            <Tabs
              value={tab}
              onValueChange={(value) => onTabChange(value as ImageDrawerTab)}
              className="gap-0"
            >
              <TabsList variant="underline" className="justify-start px-2">
                {IMAGE_DRAWER_TABS.map((key) => (
                  <TabsTrigger
                    key={key}
                    value={key}
                    className="px-3 py-2 text-sm"
                  >
                    {t(TAB_LABEL[key])}
                  </TabsTrigger>
                ))}
              </TabsList>
              <TabsContent value="versions" className="p-4">
                <ul className="divide-y rounded-lg border">
                  {versions.map((v) => (
                    <li key={v.id} className="space-y-1 p-3">
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-mono text-xs">{v.id}</span>
                        <BuildStatusBadge
                          status={v.status}
                          latest={latestAvailable?.id === v.id}
                        />
                      </div>
                      <div className="text-muted-foreground flex flex-wrap gap-x-3 text-xs">
                        <span>{v.source.reference}</span>
                        <span className="font-mono">
                          {shortDigest(v.sourceDigest)}
                        </span>
                        <LocalDateTime value={v.createdAt} />
                      </div>
                      {v.status === "failed" ? (
                        <FailureReason reason={v.failureReason} />
                      ) : null}
                      {isBuildInProgress(v.status) ? (
                        <span className="text-muted-foreground block text-xs">
                          {t("sandboxGroupPages.imageImportingHelp")}
                        </span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </TabsContent>
              <TabsContent value="details" className="p-4">
                <dl className="divide-y">
                  <DetailRow
                    label={t("sandboxGroupPages.detailLatestAvailable")}
                  >
                    {latestAvailable ? (
                      <span className="font-mono text-xs">
                        {latestAvailable.id}
                      </span>
                    ) : (
                      "—"
                    )}
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.detailImageSource")}>
                    {versions[0].source.repository}
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.detailImage")}>
                    <span className="font-mono text-xs">
                      {sourceRef(versions[0].source)}
                    </span>
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.detailDigest")}>
                    <span className="font-mono text-xs break-all">
                      {(latestAvailable ?? versions[0]).sourceDigest}
                    </span>
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.detailVersionId")}>
                    <span className="font-mono text-xs">
                      {(latestAvailable ?? versions[0]).id}
                    </span>
                  </DetailRow>
                </dl>
              </TabsContent>
              <TabsContent value="sandboxes" className="p-4">
                <StartedSandboxesList
                  groupId={group.id}
                  sandboxes={started.sandboxes}
                  loading={started.loading}
                  empty={t("sandboxGroupPages.noSandboxesFromImage")}
                />
              </TabsContent>
            </Tabs>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
