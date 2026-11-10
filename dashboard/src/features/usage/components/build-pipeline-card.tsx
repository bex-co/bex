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

import { useState } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { isForbiddenError } from "@/common/lib/graphql-error";
import { usePipelineSettings } from "@/features/usage/hooks/use-pipeline-settings";
import { money } from "@/features/usage/lib/charges";
import { BuildPipelineCardSkeleton } from "./build-pipeline-card-skeleton";

/** A dollar amount with at most two decimals, as the API accepts it. */
const DOLLARS = /^\d+(\.\d{1,2})?$/;

/**
 * Workspace Settings → Build Pipeline (w6/m309), as on Render: the monthly
 * spend limit on build minutes beyond the plan's included ones. Once reached,
 * builds stop until the next month.
 */
export function BuildPipelineCard({ workspaceId }: { workspaceId: string }) {
  const { t, i18n } = useTranslations();
  const { spendLimitUsd, loading, error, saving, save } =
    usePipelineSettings(workspaceId);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const valid = DOLLARS.test(draft.trim());

  if (loading && spendLimitUsd === null) return <BuildPipelineCardSkeleton />;
  // A failed read must not pass for "no limit".
  const unreadable = error != null && spendLimitUsd === null;

  function edit() {
    setDraft(spendLimitUsd ?? "");
    setOpen(true);
  }

  async function submit() {
    if (await save(draft.trim())) setOpen(false);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.pipelineTitle")}</CardTitle>
        <CardDescription>{t("usage.pipelineDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap items-center justify-between gap-3">
        <p className={unreadable ? "text-sm text-destructive" : "text-sm"}>
          {unreadable
            ? t(
                isForbiddenError(error)
                  ? "usage.pipelineSpendLimitForbidden"
                  : "usage.pipelineSpendLimitUnreadable",
              )
            : spendLimitUsd
              ? t("usage.pipelineSpendLimitValue", {
                  limit: money(Number(spendLimitUsd), i18n.language),
                })
              : t("usage.pipelineSpendLimitNone")}
        </p>
        <div className="flex gap-2">
          {spendLimitUsd ? (
            <Button
              variant="outline"
              size="sm"
              disabled={saving}
              onClick={() => void save(null)}
            >
              {t("usage.pipelineSpendLimitRemove")}
            </Button>
          ) : null}
          <Button size="sm" disabled={saving || unreadable} onClick={edit}>
            {spendLimitUsd
              ? t("usage.pipelineSpendLimitEdit")
              : t("usage.pipelineSpendLimitSet")}
          </Button>
        </div>
      </CardContent>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("usage.pipelineSpendLimitDialogTitle")}
            </DialogTitle>
            <DialogDescription>
              {t("usage.pipelineSpendLimitDialogDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="pipeline-spend-limit">
              {t("usage.pipelineSpendLimitLabel")}
            </Label>
            <Input
              id="pipeline-spend-limit"
              inputMode="decimal"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              aria-invalid={draft !== "" && !valid}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button disabled={!valid || saving} onClick={() => void submit()}>
              {t("usage.pipelineSpendLimitSave")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
