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

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { UsageNavigation } from "../usage-navigation";

describe("UsageNavigation", () => {
  it("renders an accessible quick nav linking every billing-page section", () => {
    render(<UsageNavigation className="sticky" />);

    const navigation = screen.getByRole("navigation", {
      name: "Billing sections",
    });
    expect(navigation).toHaveClass("sticky");

    const expected: [string, string][] = [
      ["Billing", "#billing"],
      ["Estimated Cost", "#estimated-cost"],
      ["Usage", "#usage"],
      ["Compute", "#compute"],
      ["Bandwidth", "#bandwidth"],
      ["Build Minutes", "#build"],
      ["Storage", "#storage"],
      ["3-Month Trend", "#trend"],
    ];
    for (const [name, href] of expected) {
      expect(within(navigation).getByRole("link", { name })).toHaveAttribute(
        "href",
        href,
      );
    }
  });
});
