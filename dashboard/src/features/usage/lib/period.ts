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

/** Period string "YYYY-MM" for the month that is `monthsBack` months before now. */
export function periodFor(monthsBack: number): string {
  const d = new Date();
  d.setDate(1);
  d.setMonth(d.getMonth() - monthsBack);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/** Human-readable label for a "YYYY-MM" period string. */
export function periodLabel(period: string): string {
  const [year, month] = period.split("-").map(Number);
  return new Date(year!, month! - 1, 1).toLocaleString("default", {
    month: "long",
    year: "numeric",
  });
}
