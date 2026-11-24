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
