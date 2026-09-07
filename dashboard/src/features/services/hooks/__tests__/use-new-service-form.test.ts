import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { GitRuntime } from "@/features/services/lib/runtime";

const instanceTypesState: { instanceTypes: { id: string; name: string }[] } = {
  instanceTypes: [],
};
vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => instanceTypesState,
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
    instanceTypesState.instanceTypes = [];
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

  it("moves an image-tab selection back to GitHub when Static Site is chosen", () => {
    const { result } = renderHook(() => useNewServiceForm({}));
    act(() => result.current.setTab("image"));
    expect(result.current.form.tab).toBe("image");

    // A static site has no image source (ADR029) — the type switch resets the
    // tab exactly as a manual tab switch would.
    act(() => result.current.setServiceType("static_site"));
    expect(result.current.form.serviceType).toBe("static_site");
    expect(result.current.form.tab).toBe("github");
    expect(result.current.form.selectedRepo).toBeNull();

    // Image-valid types keep whatever tab is selected.
    act(() => result.current.setTab("image"));
    act(() => result.current.setServiceType("background_worker"));
    expect(result.current.form.tab).toBe("image");
  });

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

describe("useNewServiceForm plan eligibility (w6/025 paid-only workers)", () => {
  beforeEach(() => {
    instanceTypesState.instanceTypes = [
      { id: "free", name: "Free" },
      { id: "starter", name: "Starter" },
      { id: "standard", name: "Standard" },
    ];
  });

  it("defaults a background worker to the first paid tier and offers no Free", () => {
    const { result } = renderHook(() =>
      useNewServiceForm({ type: "background_worker" }),
    );
    expect(result.current.form.plan).toBe("starter");
    expect(result.current.instanceTypes.map((it) => it.id)).toEqual([
      "starter",
      "standard",
    ]);
  });

  it("never carries a Free selection into a worker submission, and restores it on switching back", () => {
    const { result } = renderHook(() => useNewServiceForm({}));
    expect(result.current.form.plan).toBe("free"); // web default: catalog head

    act(() => result.current.set({ planOverride: "free" }));
    act(() => result.current.setServiceType("background_worker"));
    expect(result.current.form.plan).toBe("starter");

    act(() => result.current.setServiceType("web_service"));
    expect(result.current.form.plan).toBe("free");
  });
});
