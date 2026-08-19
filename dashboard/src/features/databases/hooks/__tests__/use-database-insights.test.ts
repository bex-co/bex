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
  SetDatabaseParameterOverridesDocument,
} from "@/graphql/definitions";
import { useDatabaseInsights } from "@/features/databases/hooks/use-database-insights";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const setParameters = vi.fn();
const refetchOverrides = vi.fn();

describe("useDatabaseInsights parameter writes", () => {
  beforeEach(() => {
    setParameters.mockReset();
    refetchOverrides.mockReset();
    refetchOverrides.mockResolvedValue(undefined);
    mockUseQuery.mockReset();
    mockUseMutation.mockReset();
    mockUseQuery.mockImplementation((document: unknown) => ({
      data:
        document === DatabaseParameterOverridesDocument
          ? { databaseParameterOverrides: [] }
          : undefined,
      loading: false,
      error: undefined,
      refetch:
        document === DatabaseParameterOverridesDocument
          ? refetchOverrides
          : vi.fn(),
    }));
    mockUseMutation.mockReturnValue([setParameters, { loading: false }]);
  });

  it("sends the replace payload and refreshes the live view", async () => {
    setParameters.mockResolvedValue({ data: {} });
    const { result } = renderHook(() => useDatabaseInsights("db-1"));

    const saved = await result.current.saveParameters([
      { name: "work_mem", value: "16MB" },
    ]);

    expect(saved).toEqual({ ok: true });
    expect(mockUseMutation).toHaveBeenCalledWith(
      SetDatabaseParameterOverridesDocument,
    );
    expect(setParameters).toHaveBeenCalledWith({
      variables: {
        id: "db-1",
        parameters: [{ name: "work_mem", value: "16MB" }],
      },
    });
    expect(refetchOverrides).toHaveBeenCalledOnce();
    expect(mockUseQuery).toHaveBeenCalledWith(
      DatabaseParameterOverridesDocument,
      expect.objectContaining({ pollInterval: 15_000 }),
    );
  });

  it("returns the backend message without refetching after rejection", async () => {
    setParameters.mockRejectedValue(new Error("operation forbidden"));
    const { result } = renderHook(() => useDatabaseInsights("db-1"));

    const saved = await result.current.saveParameters([]);

    expect(saved).toEqual({ ok: false, error: "operation forbidden" });
    expect(refetchOverrides).not.toHaveBeenCalled();
  });
});
