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
import { PricingWithTerms } from "@/features/legal/components/agent-model-terms-link";

/**
 * The card's collapsed ready geometry, for loading and the route's pending
 * state. The title and the pricing statement with its "Terms" link are static,
 * so they render as real text and wrap exactly as the ready card's do.
 */
export function AgentModelCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="agent-model-card-skeleton">
      <CardHeader>
        <CardTitle>{t("usage.agentModelTitle")}</CardTitle>
        <CardDescription>
          <PricingWithTerms
            pricing={t("usage.agentModelPricing")}
            placeholder
          />
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-baseline justify-between gap-4">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-7 w-20" />
        </div>
        <Skeleton className="h-8 w-48" />
      </CardContent>
    </Card>
  );
}
