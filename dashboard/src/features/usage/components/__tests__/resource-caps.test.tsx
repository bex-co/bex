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

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResourceCaps } from "@/features/usage/components/resource-caps";

describe("ResourceCaps", () => {
  it("shows capped resource counts and warns at 80 percent", () => {
    render(
      <ResourceCaps
        limits={{
          services: { used: 20, limit: 25 },
          postgres: { used: 1, limit: 2 },
          keyValues: { used: 0, limit: 1 },
        }}
      />,
    );

    expect(screen.getByText("Resource limits")).toBeInTheDocument();
    expect(screen.getByText("20 of 25 used")).toBeInTheDocument();
    expect(screen.getByText("1 of 2 used")).toBeInTheDocument();
    expect(screen.getByText("0 of 1 used")).toBeInTheDocument();
    expect(screen.getByText("Near limit")).toBeInTheDocument();
    expect(screen.getAllByRole("progressbar")).toHaveLength(3);
  });

  it("hides the card when every zero limit means unlimited", () => {
    const { container } = render(
      <ResourceCaps
        limits={{
          services: { used: 12, limit: 0 },
          postgres: { used: 3, limit: 0 },
          keyValues: { used: 4, limit: 0 },
        }}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("omits unlimited kinds while retaining configured caps", () => {
    render(
      <ResourceCaps
        limits={{
          services: { used: 12, limit: 0 },
          postgres: { used: 1, limit: 1 },
          keyValues: { used: 4, limit: 0 },
        }}
      />,
    );

    expect(screen.queryByText("Services")).not.toBeInTheDocument();
    expect(screen.getByText("Postgres")).toBeInTheDocument();
    expect(screen.queryByText("Key Value")).not.toBeInTheDocument();
    expect(screen.getByText("1 of 1 used")).toBeInTheDocument();
  });
});
