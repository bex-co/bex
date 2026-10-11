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
import { buildCreateServiceInput } from "@/features/services/lib/create-service-input";

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

// w4/226: Public Git URL → Existing Image kept a typed Root Directory hidden
// in the draft and sent it with the image create, which the API refuses.
describe("useNewServiceForm source switch keeps Git-only Root Directory local", () => {
  it("omits the staged root from an image create and restores it on return", () => {
    const { result } = renderHook(() => useNewServiceForm({}));
    act(() => result.current.setTab("git"));
    act(() =>
      result.current.set({
        gitUrl: "https://github.com/bex-co/bex",
        rootDir: "examples/hello-go",
      }),
    );

    act(() => result.current.setTab("image"));
    act(() =>
      result.current.set({
        image: "docker.io/library/busybox:1.37",
        imageCommand: "httpd -f",
      }),
    );
    const image = JSON.parse(
      JSON.stringify(buildCreateServiceInput(result.current.form)),
    );
    expect(image).not.toHaveProperty("rootDir");
    expect(image).toMatchObject({
      image: "docker.io/library/busybox:1.37",
      startCommand: "httpd -f",
    });

    act(() => result.current.setTab("git"));
    expect(result.current.form.rootDir).toBe("examples/hello-go");
    expect(buildCreateServiceInput(result.current.form).rootDir).toBe(
      "examples/hello-go",
    );
  });
});

// w4/240: a GitHub/Image round trip cleared the Public Git Branch while the
// URL, Root Directory and Publish Directory survived.
describe("useNewServiceForm Public Git Branch draft", () => {
  const URL = "https://github.com/bex-co/bex";
  const BRANCH = "copilot/fix-inconsistent-outdated-docs";

  function renderGitDraft(branch: string) {
    const { result } = renderHook(() =>
      useNewServiceForm({ type: "static_site" }),
    );
    act(() => result.current.setTab("git"));
    act(() =>
      result.current.set({
        gitUrl: URL,
        branch,
        rootDir: "examples/static-site",
        publishPath: ".",
      }),
    );
    return result;
  }

  it.each(["github", "image"] as const)(
    "restores the branch after a %s round trip with the same URL",
    (away) => {
      const result = renderGitDraft(BRANCH);
      act(() => result.current.setTab(away));
      act(() => result.current.setTab("git"));
      expect(result.current.form).toMatchObject({
        gitUrl: URL,
        branch: BRANCH,
        rootDir: "examples/static-site",
        publishPath: ".",
      });
      expect(
        JSON.parse(
          JSON.stringify(buildCreateServiceInput(result.current.form)),
        ),
      ).toMatchObject({ repo: URL, branch: BRANCH });
    },
  );

  it("keeps an intentionally blank branch blank", () => {
    const result = renderGitDraft("");
    act(() => result.current.setTab("github"));
    act(() => result.current.setTab("git"));
    expect(result.current.form.branch).toBe("");
  });

  it("never carries the Git branch into a GitHub repository selection", () => {
    const result = renderGitDraft(BRANCH);
    act(() => result.current.setTab("github"));
    expect(result.current.form.branch).toBe("");
    act(() => result.current.set({ selectedRepo: REPO, branch: "main" }));
    expect(buildCreateServiceInput(result.current.form)).toMatchObject({
      repo: REPO.cloneUrl,
      branch: "main",
    });
  });

  it("does not restore one repository's branch to a different URL", () => {
    const result = renderGitDraft(BRANCH);
    act(() => result.current.setTab("github"));
    act(() => result.current.set({ gitUrl: "https://github.com/acme/other" }));
    act(() => result.current.setTab("git"));
    expect(result.current.form.branch).toBe("");
  });
});
