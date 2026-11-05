/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  DatabaseParameterOverridesDocument,
  DatabaseParameterSpecDocument,
} from "@/graphql/definitions";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { useDatabaseInsights } from "@/features/databases/hooks/use-database-insights";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("useDatabaseInsights parameter reads", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    mockUseQuery.mockImplementation((document: unknown) => ({
      data:
        document === DatabaseParameterSpecDocument
          ? {
              databaseParameterSpec: [{ name: "work_mem", value: "8MB" }, null],
            }
          : undefined,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }));
  });

  it("refreshes declared observations on the same visible-page cadence", () => {
    renderHook(() => useDatabaseInsights("db-1"));
    for (const document of [
      DatabaseParameterSpecDocument,
      DatabaseParameterOverridesDocument,
    ]) {
      expect(mockUseQuery).toHaveBeenCalledWith(
        document,
        expect.objectContaining({
          pollInterval: 15_000,
          skipPollAttempt: skipPollWhenHidden,
        }),
      );
    }
  });

  // parameterOverrides was retired (w2/066): the hook reads the declared set
  // and exposes no write.
  it("exposes the declared set read-only, without null rows", () => {
    const { result } = renderHook(() => useDatabaseInsights("db-1"));
    expect(result.current.parameterSpec).toEqual([
      { name: "work_mem", value: "8MB" },
    ]);
    expect(result.current).not.toHaveProperty("saveParameters");
    expect(result.current).not.toHaveProperty("saving");
  });
});
