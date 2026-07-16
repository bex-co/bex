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

import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useResourceLimits } from "@/features/usage/hooks/use-resource-limits";
import {
  createLoadingQueryResult,
  createSuccessQueryResult,
} from "@/test/mocks/apollo";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

describe("useResourceLimits", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    currentWorkspaceId = "tea-1";
  });

  it("normalizes all three cap counters", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        workspaceLimits: {
          services: { used: 3, limit: 25 },
          postgres: { used: 1, limit: 1 },
          keyValues: { used: 0, limit: 1 },
        },
      }),
    );

    const { result } = renderHook(() => useResourceLimits());

    expect(result.current.limits).toEqual({
      services: { used: 3, limit: 25 },
      postgres: { used: 1, limit: 1 },
      keyValues: { used: 0, limit: 1 },
    });
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { ownerId: "tea-1" },
        pollInterval: 15_000,
      }),
    );
  });

  it("waits for the workspace selection before querying", () => {
    currentWorkspaceId = null;
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    const { result } = renderHook(() => useResourceLimits());

    expect(result.current.loading).toBe(true);
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ skip: true }),
    );
  });
});
