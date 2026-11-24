import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import {
  CreateActionBarSkeleton,
  CreateFieldSkeleton,
  CreateForm,
} from "@/common/components/create-form";
import { Card, CardContent } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Breakpoint-matched skeletons for `/build-sources` and `/build-sources/new`
 * (w2/m613). Kept in a skeleton-only module so route pending components stay
 * out of the lazy page chunks (w3/066, `route-pending-chunk.test.ts`).
 */

export function BuildSourcesListContentSkeleton({
  rows = 3,
}: {
  rows?: number;
}) {
  return (
    <div className="space-y-4" aria-hidden="true">
      {/* Search + source-kind filter toolbar */}
      <div
        data-skeleton-region="filters"
        className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
      >
        <Skeleton className="h-11 w-full rounded-md sm:h-9 sm:max-w-md" />
        <div className="flex flex-wrap items-center gap-1.5">
          <Skeleton className="h-9 w-14 rounded-md" />
          <Skeleton className="h-9 w-14 rounded-md" />
          <Skeleton className="h-9 w-28 rounded-md" />
        </div>
      </div>

      <div data-skeleton-region="build-sources-list" className="space-y-4">
        {/* Desktop table preview (md+) */}
        <div className="hidden overflow-hidden rounded-lg border md:block">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[22%]">
                  <Skeleton className="h-3.5 w-14" />
                </TableHead>
                <TableHead className="w-[28%]">
                  <Skeleton className="h-3.5 w-16" />
                </TableHead>
                <TableHead className="w-[13%]">
                  <Skeleton className="h-3.5 w-14" />
                </TableHead>
                <TableHead className="w-[13%]">
                  <Skeleton className="h-3.5 w-20" />
                </TableHead>
                <TableHead className="w-[12%]">
                  <Skeleton className="h-3.5 w-14" />
                </TableHead>
                <TableHead className="w-[12%] text-right">
                  <Skeleton className="ml-auto h-3.5 w-16" />
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: rows }).map((_, idx) => (
                <TableRow key={idx}>
                  <TableCell className="py-3.5">
                    <div className="space-y-1.5">
                      <Skeleton className="h-4 w-32" />
                      <Skeleton className="h-3 w-36" />
                    </div>
                  </TableCell>
                  <TableCell className="py-3.5">
                    <div className="space-y-1.5">
                      <div className="flex items-center gap-1.5">
                        <Skeleton className="h-5 w-12 rounded-full" />
                        <Skeleton className="h-5 w-14 rounded-full" />
                      </div>
                      <Skeleton className="h-3.5 w-44" />
                    </div>
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Skeleton className="h-4 w-16" />
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Skeleton className="h-5 w-20 rounded-full" />
                  </TableCell>
                  <TableCell className="py-3.5 text-right">
                    <Skeleton className="ml-auto h-4 w-20" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        {/* Narrow-mobile stacked cards preview (< md, 390px) */}
        <div className="space-y-3 md:hidden">
          {Array.from({ length: rows }).map((_, idx) => (
            <Card key={idx} className="gap-3 py-4">
              <CardContent className="space-y-3 px-4">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 space-y-1">
                    <Skeleton className="h-4 w-36" />
                    <Skeleton className="h-3 w-40" />
                  </div>
                  <Skeleton className="h-5 w-14 shrink-0 rounded-full" />
                </div>
                <Skeleton className="h-3.5 w-48" />
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-2.5">
                  <Skeleton className="h-3.5 w-32" />
                  <Skeleton className="h-3.5 w-20" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </div>
  );
}

export function BuildSourcesListRouteSkeleton() {
  const { t } = useTranslations();
  return (
    <DashboardLayout>
      <PageFrame
        pending="build-sources-list"
        header={
          <PageHeader
            title={t("buildSources.pageTitle")}
            description={t("buildSources.pageDescription")}
            actions={<Skeleton className="h-9 w-36 rounded-md" />}
          />
        }
      >
        <BuildSourcesListContentSkeleton />
      </PageFrame>
    </DashboardLayout>
  );
}

export function CreateBuildSourceRouteSkeleton() {
  const { t } = useTranslations();
  return (
    <DashboardLayout>
      <PageFrame
        width="standard"
        pending="build-source-create"
        header={
          <PageHeader
            leading={<Skeleton className="size-9 rounded-md" />}
            title={t("buildSources.createTitle")}
            description={t("buildSources.createDescription")}
          />
        }
        footer={
          <CreateActionBarSkeleton
            submitLabel={t("buildSources.createSubmit")}
            cancelLabel={t("common.cancel")}
            summary={false}
          />
        }
      >
        <CreateForm>
          <div data-skeleton-region="source-picker">
            <CreateFieldSkeleton descriptionLines={1}>
              <div className="space-y-3">
                <div className="flex gap-2 border-b pb-2">
                  <Skeleton className="h-8 w-24 rounded-md" />
                  <Skeleton className="h-8 w-24 rounded-md" />
                  <Skeleton className="h-8 w-28 rounded-md" />
                </div>
                <Skeleton className="h-11 w-full rounded-md sm:h-9" />
                <Skeleton className="h-40 w-full rounded-lg" />
              </div>
            </CreateFieldSkeleton>
          </div>

          <div data-skeleton-region="details" className="space-y-8">
            <CreateFieldSkeleton descriptionLines={1} />
            <CreateFieldSkeleton descriptionLines={1} />
          </div>

          <div data-skeleton-region="build-settings" className="space-y-8">
            <CreateFieldSkeleton descriptionLines={1} />
            <CreateFieldSkeleton descriptionLines={1} />
            <CreateFieldSkeleton descriptionLines={1} />
            <CreateFieldSkeleton descriptionLines={1} />
            <Skeleton className="h-12 w-full rounded-lg" />
          </div>

          <div data-skeleton-region="link-services">
            <div className="space-y-3 rounded-lg border p-4 sm:p-6">
              <Skeleton className="h-5 w-44" />
              <Skeleton className="h-4 w-72 max-w-full" />
              <div className="space-y-2 pt-1">
                <Skeleton className="h-11 w-full rounded-md" />
                <Skeleton className="h-11 w-full rounded-md" />
              </div>
            </div>
          </div>
        </CreateForm>
      </PageFrame>
    </DashboardLayout>
  );
}

export function BuildSourceBuildsTabSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div data-skeleton-region="builds-list" className="space-y-4" aria-hidden="true">
      {/* Desktop builds table (md+) */}
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[24%]">
                <Skeleton className="h-3.5 w-16" />
              </TableHead>
              <TableHead className="w-[16%]">
                <Skeleton className="h-3.5 w-14" />
              </TableHead>
              <TableHead className="w-[24%]">
                <Skeleton className="h-3.5 w-24" />
              </TableHead>
              <TableHead className="w-[12%]">
                <Skeleton className="h-3.5 w-16" />
              </TableHead>
              <TableHead className="w-[12%]">
                <Skeleton className="h-3.5 w-14" />
              </TableHead>
              <TableHead className="w-[12%] text-right">
                <Skeleton className="ml-auto h-3.5 w-16" />
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: rows }).map((_, idx) => (
              <TableRow key={idx}>
                <TableCell className="py-3.5">
                  <Skeleton className="h-4 w-36" />
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-5 w-20 rounded-full" />
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-4 w-32" />
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-4 w-12" />
                </TableCell>
                <TableCell className="py-3.5">
                  <Skeleton className="h-5 w-16 rounded-full" />
                </TableCell>
                <TableCell className="py-3.5 text-right">
                  <Skeleton className="ml-auto h-4 w-20" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Narrow-mobile stacked cards (< md, 390px) */}
      <div className="space-y-3 md:hidden">
        {Array.from({ length: rows }).map((_, idx) => (
          <Card key={idx} className="gap-2 py-4">
            <CardContent className="space-y-2.5 px-4">
              <div className="flex items-center justify-between gap-2">
                <Skeleton className="h-4 w-36" />
                <Skeleton className="h-5 w-20 rounded-full" />
              </div>
              <Skeleton className="h-3.5 w-44" />
              <div className="flex items-center justify-between gap-2 border-t pt-2">
                <Skeleton className="h-3.5 w-24" />
                <Skeleton className="h-3.5 w-20" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

export function BuildSourceDetailRouteSkeleton() {
  return (
    <DashboardLayout>
      <PageFrame
        pending="build-source-detail"
        header={
          <div className="space-y-4" aria-hidden="true">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="flex min-w-0 items-start gap-2.5">
                <Skeleton className="mt-0.5 size-8 shrink-0 rounded-md" />
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Skeleton className="h-7 w-44" />
                    <Skeleton className="h-5 w-14 rounded-full" />
                    <Skeleton className="h-5 w-16 rounded-full" />
                  </div>
                  <div className="flex flex-wrap items-center gap-3">
                    <Skeleton className="h-3.5 w-48" />
                    <Skeleton className="h-3.5 w-36" />
                  </div>
                </div>
              </div>
              <Skeleton className="h-9 w-32 shrink-0 rounded-md" />
            </div>

            <div
              data-skeleton-region="detail-tabs"
              className="flex gap-5 border-b pb-2.5"
            >
              <Skeleton className="h-4 w-14" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-16" />
            </div>
          </div>
        }
      >
        <BuildSourceBuildsTabSkeleton />
      </PageFrame>
    </DashboardLayout>
  );
}

export function BuildDetailRouteSkeleton() {
  return (
    <div className="space-y-6" aria-hidden="true">
      <div data-skeleton-region="build-header" className="space-y-4">
        <Skeleton className="h-4 w-28" />
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1.5">
            <Skeleton className="h-6 w-52" />
            <Skeleton className="h-4 w-36" />
          </div>
          <Skeleton className="h-6 w-24 rounded-full" />
        </div>
        <div className="grid grid-cols-1 gap-3 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-4">
          {[0, 1, 2, 3].map((idx) => (
            <div key={idx} className="space-y-1.5">
              <Skeleton className="h-3 w-16" />
              <Skeleton className="h-4 w-28" />
            </div>
          ))}
        </div>
      </div>

      <div
        data-skeleton-region="build-fanout"
        className="space-y-3 rounded-lg border p-4"
      >
        <Skeleton className="h-5 w-36" />
        <Skeleton className="h-10 w-full rounded-md" />
      </div>

      <div data-skeleton-region="build-logs" className="space-y-3">
        <div className="flex items-center gap-2">
          <Skeleton className="h-9 flex-1 rounded-md" />
          <Skeleton className="size-9 rounded-md" />
          <Skeleton className="size-9 rounded-md" />
        </div>
        <div className="space-y-2 rounded-lg border p-4">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-4 w-4/5" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      </div>
    </div>
  );
}

export function BuildSourceServicesTabSkeleton({
  rows = 2,
}: {
  rows?: number;
}) {
  return (
    <div
      data-skeleton-region="linked-services"
      className="space-y-4"
      aria-hidden="true"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1.5">
          <Skeleton className="h-5 w-36" />
          <Skeleton className="h-4 w-64 max-w-full" />
        </div>
        <Skeleton className="h-9 w-40 rounded-md" />
      </div>
      <div className="space-y-3">
        {Array.from({ length: rows }).map((_, idx) => (
          <Card key={idx} className="py-4">
            <CardContent className="flex flex-wrap items-center justify-between gap-3 px-4">
              <div className="space-y-1.5">
                <Skeleton className="h-4 w-36" />
                <div className="flex items-center gap-2">
                  <Skeleton className="h-5 w-20 rounded-full" />
                  <Skeleton className="h-5 w-16 rounded-full" />
                </div>
              </div>
              <Skeleton className="h-8 w-28 rounded-md" />
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

export function BuildSourceEnvironmentTabSkeleton() {
  return (
    <div
      data-skeleton-region="build-environment"
      className="space-y-6"
      aria-hidden="true"
    >
      <Skeleton className="h-4 w-80 max-w-full" />
      {[0, 1, 2].map((idx) => (
        <Card key={idx} className="py-5">
          <CardContent className="space-y-4 px-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="space-y-1.5">
                <Skeleton className="h-5 w-44" />
                <Skeleton className="h-4 w-72 max-w-full" />
              </div>
              <Skeleton className="h-9 w-24 rounded-md" />
            </div>
            <div className="space-y-2.5">
              <Skeleton className="h-10 w-full rounded-md" />
              <Skeleton className="h-10 w-full rounded-md" />
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

export function BuildSourceSettingsTabSkeleton() {
  return (
    <div
      data-skeleton-region="build-settings"
      className="space-y-6"
      aria-hidden="true"
    >
      <Card className="py-5">
        <CardContent className="space-y-4 px-5">
          <Skeleton className="h-5 w-40" />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {[0, 1, 2].map((idx) => (
              <div key={idx} className="space-y-1.5">
                <Skeleton className="h-3.5 w-20" />
                <Skeleton className="h-4 w-32" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
      <Card className="py-5">
        <CardContent className="space-y-5 px-5">
          <Skeleton className="h-5 w-44" />
          <div className="space-y-4">
            <Skeleton className="h-10 w-full rounded-md" />
            <Skeleton className="h-10 w-full rounded-md" />
            <Skeleton className="h-10 w-full rounded-md" />
          </div>
          <Skeleton className="h-9 w-32 rounded-md" />
        </CardContent>
      </Card>
      <Card className="py-5">
        <CardContent className="space-y-3 px-5">
          <Skeleton className="h-5 w-36" />
          <Skeleton className="h-4 w-72 max-w-full" />
          <Skeleton className="h-9 w-40 rounded-md" />
        </CardContent>
      </Card>
    </div>
  );
}
