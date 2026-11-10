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
import { UsageMeter } from "@/common/components/usage-meter";

const NEAR_LIMIT_RATIO = 0.8;

/**
 * One of Render's "Monthly included usage" allowances (Free instance hours,
 * included build minutes): what was used against what the month includes,
 * with a note near the allowance and once it is spent.
 */
export function IncludedUsageCard({
  title,
  description,
  usedText,
  ratio,
  nearText,
  spentText,
}: {
  title: string;
  description: string;
  usedText: string;
  ratio: number;
  nearText: string;
  spentText: string;
}) {
  const spent = ratio >= 1;
  const near = ratio >= NEAR_LIMIT_RATIO;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <p className="text-sm font-medium">{usedText}</p>
        <UsageMeter
          label={title}
          fraction={ratio}
          level={spent ? "reached" : near ? "warning" : "ok"}
          valueText={usedText}
        />
        {spent ? (
          <p className="text-xs text-destructive">{spentText}</p>
        ) : near ? (
          <p className="text-xs text-muted-foreground">{nearText}</p>
        ) : null}
      </CardContent>
    </Card>
  );
}
