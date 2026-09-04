import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { GitRuntime } from "@/features/services/lib/runtime";

vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({ instanceTypes: [] }),
}));

vi.mock("@/features/services/hooks/use-service-name-draft", () => ({
  useServiceNameDraft: () => ({
    name: "service",
    nameValid: true,
    nameTaken: false,
    nameSuggestion: null,
    checkingName: false,
    editName: vi.fn(),
    acceptSuggestion: vi.fn(),
  }),
}));

let detectionFailure = false;
vi.mock("@/features/services/hooks/use-repo-runtime-detection", () => ({
  useRepoRuntimeDetection: ({
    repo,
    rootDir,
  }: {
    repo: string | null;
    rootDir: string;
  }) => {
    if (!repo || detectionFailure || rootDir === "unknown") {
      return null;
    }
    const runtime: GitRuntime = rootDir === "services/python" ? "python" : "go";
    return runtime;
  },
}));

import { useNewServiceForm } from "@/features/services/hooks/use-new-service-form";

const REPO: RepoView = {
  id: 1,
  fullName: "acme/mono",
  private: false,
  defaultBranch: "main",
  htmlUrl: "https://github.com/acme/mono",
  cloneUrl: "https://github.com/acme/mono.git",
  accountLogin: "acme",
};

describe("useNewServiceForm runtime detection", () => {
  beforeEach(() => {
    detectionFailure = false;
  });

  it("auto-selects on repo pick and re-infers when Root Directory changes", async () => {
    const { result } = renderHook(() => useNewServiceForm({}));

    act(() => result.current.set({ selectedRepo: REPO, branch: "main" }));
    await waitFor(() => expect(result.current.form.runtime).toBe("go"));
    expect(result.current.form.buildCommand).toBe("go build -o app .");
    expect(result.current.form.startCommand).toBe("./app");

    act(() => result.current.set({ rootDir: "services/python" }));
    await waitFor(() => expect(result.current.form.runtime).toBe("python"));
    expect(result.current.form.buildCommand).toBe(
      "pip install -r requirements.txt",
    );
  });

  it("never clobbers an explicit runtime choice with a later result", async () => {
    const { result } = renderHook(() => useNewServiceForm({}));
    act(() => result.current.set({ selectedRepo: REPO, branch: "main" }));
    await waitFor(() => expect(result.current.form.runtime).toBe("go"));

    act(() => result.current.build.setRuntime("ruby"));
    act(() => result.current.set({ rootDir: "services/python" }));
    await waitFor(() =>
      expect(result.current.form.rootDir).toBe("services/python"),
    );
    expect(result.current.form.runtime).toBe("ruby");
    expect(result.current.form.buildCommand).toBe("bundle install");
  });

  it.each([
    ["build command", "setBuildCommand", "make custom", "buildCommand"],
    ["start command", "setStartCommand", "./custom", "startCommand"],
  ] as const)(
    "never clobbers an explicit %s with a later result",
    async (_label, setter, customValue, field) => {
      const { result } = renderHook(() => useNewServiceForm({}));
      act(() => result.current.set({ selectedRepo: REPO, branch: "main" }));
      await waitFor(() => expect(result.current.form.runtime).toBe("go"));

      act(() => result.current.build[setter](customValue));
      act(() => result.current.set({ rootDir: "services/python" }));
      await waitFor(() =>
        expect(result.current.form.rootDir).toBe("services/python"),
      );
      expect(result.current.form.runtime).toBe("go");
      expect(result.current.form[field]).toBe(customValue);
    },
  );

  it("leaves the current selection untouched when detection fails", async () => {
    const { result, rerender } = renderHook(() => useNewServiceForm({}));
    act(() => result.current.set({ selectedRepo: REPO, branch: "main" }));
    await waitFor(() => expect(result.current.form.runtime).toBe("go"));

    detectionFailure = true;
    act(() => result.current.set({ rootDir: "unknown" }));
    rerender();
    expect(result.current.form.runtime).toBe("go");
    expect(result.current.form.buildCommand).toBe("go build -o app .");
  });
});
