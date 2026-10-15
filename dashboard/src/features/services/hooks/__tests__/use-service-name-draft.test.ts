import { act, renderHook } from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { SourceTab } from "@/features/services/components/service-source-picker";

// Names the availability check reports taken, each with its free suggestion.
const takenNames = new Map<string, string>();
vi.mock("@/features/services/hooks/use-service-name-availability", () => ({
  useServiceNameAvailability: (name: string) => {
    const suggestion = takenNames.get(name) ?? null;
    return { taken: suggestion !== null, suggestion, checking: false };
  },
}));

import { useServiceNameDraft } from "@/features/services/hooks/use-service-name-draft";

function repo(fullName: string, defaultBranch = "main"): RepoView {
  return {
    id: fullName.length,
    fullName,
    private: false,
    defaultBranch,
    htmlUrl: `https://github.com/${fullName}`,
    cloneUrl: `https://github.com/${fullName}.git`,
    accountLogin: fullName.split("/")[0],
  };
}

interface Source {
  tab: SourceTab;
  selectedRepo: RepoView | null;
  gitUrl: string;
  image: string;
}

const EMPTY: Source = {
  tab: "github",
  selectedRepo: null,
  gitUrl: "",
  image: "",
};

// The create form keeps the branch in its own state and only fills a blank
// one, exactly as useNewServiceForm wires `onRepoDefaultBranch`.
function useDraftWithBranch(source: Source) {
  const [branch, setBranch] = useState("");
  const draft = useServiceNameDraft({
    ...source,
    onRepoDefaultBranch: (next) => setBranch((current) => current || next),
  });
  return { ...draft, branch };
}

function renderDraft(initial: Source = EMPTY) {
  return renderHook((source: Source) => useDraftWithBranch(source), {
    initialProps: initial,
  });
}

beforeEach(() => {
  takenNames.clear();
});

describe("useServiceNameDraft", () => {
  it("fills the name and branch from a picked repository", () => {
    const { result, rerender } = renderDraft();
    expect(result.current.name).toBe("");

    rerender({ ...EMPTY, selectedRepo: repo("acme/web-frontend", "trunk") });
    expect(result.current.name).toBe("web-frontend");
    expect(result.current.branch).toBe("trunk");
  });

  it("fills from a repository already picked on the first render", () => {
    const { result } = renderDraft({
      ...EMPTY,
      selectedRepo: repo("acme/api-service"),
    });
    expect(result.current.name).toBe("api-service");
    expect(result.current.branch).toBe("main");
  });

  it("fills from a Git URL and from an image", () => {
    const { result, rerender } = renderDraft();
    rerender({
      ...EMPTY,
      tab: "git",
      gitUrl: "https://example.com/acme/Docs.git",
    });
    expect(result.current.name).toBe("docs");

    rerender({ ...EMPTY, tab: "image", image: "ghcr.io/acme/worker:1.2" });
    expect(result.current.name).toBe("worker");
  });

  it("keeps a typed name across source changes until it is cleared", () => {
    const { result, rerender } = renderDraft({
      ...EMPTY,
      selectedRepo: repo("acme/web-frontend"),
    });
    act(() => result.current.editName("my-web"));
    rerender({ ...EMPTY, selectedRepo: repo("acme/api-service") });
    expect(result.current.name).toBe("my-web");

    // Clearing a typed name hands the field back to the current source.
    act(() => result.current.editName(""));
    expect(result.current.name).toBe("api-service");
  });

  it("swaps a taken auto-filled name for the free suggestion, never a typed one", () => {
    takenNames.set("web-frontend", "web-frontend-2");
    const { result, rerender } = renderDraft();

    rerender({ ...EMPTY, selectedRepo: repo("acme/web-frontend") });
    expect(result.current.name).toBe("web-frontend-2");

    act(() => result.current.editName("web-frontend"));
    expect(result.current.name).toBe("web-frontend");
    expect(result.current.nameTaken).toBe(true);
    expect(result.current.nameSuggestion).toBe("web-frontend-2");
  });
});
