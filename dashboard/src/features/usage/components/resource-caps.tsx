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

import { AlertTriangle } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useResourceLimits,
  type ResourceCap,
  type ResourceLimits,
} from "@/features/usage/hooks/use-resource-limits";

interface ResourceCapsProps {
  limits: ResourceLimits | null;
}

const NEAR_LIMIT_RATIO = 0.8;

function CapTile({ label, cap }: { label: string; cap: ResourceCap }) {
  const { t } = useTranslations();
  const ratio = cap.limit > 0 ? cap.used / cap.limit : 0;
  const percent = Math.min(100, Math.max(0, ratio * 100));
  const nearLimit = ratio >= NEAR_LIMIT_RATIO;

  return (
    <div className="overflow-hidden rounded-lg border">
      <div className="space-y-1 p-3">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="flex items-baseline gap-1.5">
          <span className="text-xl font-semibold tabular-nums">{cap.used}</span>
          <span className="text-sm text-muted-foreground tabular-nums">
            / {cap.limit}
          </span>
          {nearLimit && (
            <span className="ml-auto flex items-center gap-1 text-xs font-medium text-amber-700 dark:text-amber-400">
              <AlertTriangle aria-hidden="true" className="size-3.5" />
              {t("usage.resourceCapsNearLimit")}
            </span>
          )}
        </p>
      </div>
      {/* A hairline along the card's bottom edge rather than a bar competing
          with the number — the count is the fact, the fill is the context. */}
      <div
        className="h-0.5 bg-muted"
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={cap.limit}
        aria-valuenow={cap.used}
      >
        <div
          className={cn(
            "h-full transition-[width]",
            nearLimit ? "bg-amber-500" : "bg-primary",
          )}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
}

export function ResourceCaps({ limits }: ResourceCapsProps) {
  const { t } = useTranslations();
  if (!limits) return null;

  const capped = [
    {
      key: "services",
      label: t("usage.resourceCapsServices"),
      cap: limits.services,
    },
    {
      key: "postgres",
      label: t("usage.resourceCapsPostgres"),
      cap: limits.postgres,
    },
    {
      key: "keyValues",
      label: t("usage.resourceCapsKeyValues"),
      cap: limits.keyValues,
    },
  ].filter(({ cap }) => cap.limit > 0);

  // limit=0 is the backend's explicit unlimited value. If every resource is
  // unlimited, omit the whole card instead of displaying misleading 0/0s.
  if (capped.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.includedUsageTitle")}</CardTitle>
        <CardDescription>{t("usage.includedUsageDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-3">
        {capped.map(({ key, label, cap }) => (
          <CapTile key={key} label={label} cap={cap} />
        ))}
      </CardContent>
    </Card>
  );
}

export function WorkspaceResourceCaps() {
  const { limits, error } = useResourceLimits();
  // Cap visibility is supplemental. A transient failure must not block the
  // metered usage page; the create surfaces still enforce and explain caps.
  return error ? null : <ResourceCaps limits={limits} />;
}
