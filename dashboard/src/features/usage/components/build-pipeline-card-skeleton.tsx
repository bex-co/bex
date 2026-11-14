// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/** The card's static header, shared by the ready card and its skeleton. */
export function BuildPipelineCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("usage.pipelineTitle")}</CardTitle>
      <CardDescription>{t("usage.pipelineDescription")}</CardDescription>
    </CardHeader>
  );
}

/**
 * BuildPipelineCard's pending shape (w6/m309). The header is static copy, so
 * it renders as the ready card's text and wraps alike at every width; below
 * it, the limit line (one text-sm line) and the action buttons (size sm).
 */
export function BuildPipelineCardSkeleton() {
  return (
    <Card data-testid="build-pipeline-skeleton">
      <BuildPipelineCardHeader />
      <CardContent className="flex flex-wrap items-center justify-between gap-3">
        <Skeleton className="h-5 w-44" />
        <Skeleton className="h-8 w-28" />
      </CardContent>
    </Card>
  );
}
