import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import {
  FILES_PAGE_SIZE,
  SessionFilesTable,
} from "@/features/agent-sessions/components/session-files-sheet";

type Row = {
  id: string;
  direction: string;
  turn: number | null;
  name: string;
  mediaType: string;
  size: number;
  createdAt: string;
};

const state: {
  data: { agentSessionAttachments: Row[] } | undefined;
  loading: boolean;
  error: Error | undefined;
} = { data: undefined, loading: false, error: undefined };
const fetchMore = vi.fn();
const query = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({ ...state, fetchMore }),
  useApolloClient: () => ({ query }),
}));

function row(i: number, direction = "input"): Row {
  return {
    id: `att-${i}`,
    direction,
    turn: 1,
    name: `file-${i}.txt`,
    mediaType: "text/plain",
    size: 1024,
    createdAt: "2026-11-03T00:00:00Z",
  };
}

const originalLocation = window.location;
beforeEach(() => {
  state.data = undefined;
  state.loading = false;
  state.error = undefined;
  fetchMore.mockReset();
  query.mockReset();
});
afterEach(() => {
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
  });
});

describe("SessionFilesTable", () => {
  it("previews the ready table's geometry while loading", () => {
    state.loading = true;
    const { unmount } = render(<SessionFilesTable sessionId="ags-1" />);
    const skeleton = screen.getByTestId("session-files-skeleton");
    const skeletonHead = within(skeleton)
      .getAllByRole("columnheader")
      .map((h) => h.textContent);
    const skeletonRowClass = within(skeleton).getAllByRole("row")[1].className;
    unmount();

    state.loading = false;
    state.data = { agentSessionAttachments: [row(1)] };
    render(<SessionFilesTable sessionId="ags-1" />);
    const table = screen.getByTestId("session-files-table");
    expect(
      within(table)
        .getAllByRole("columnheader")
        .map((h) => h.textContent),
    ).toEqual(skeletonHead);
    expect(within(table).getAllByRole("row")[1].className).toBe(
      skeletonRowClass,
    );
  });

  it("lists inputs and agent outputs with who added them", () => {
    state.data = {
      agentSessionAttachments: [row(1, "input"), row(2, "output")],
    };
    render(<SessionFilesTable sessionId="ags-1" />);
    const [, first, second] = screen.getAllByRole("row");
    expect(within(first).getByText("file-1.txt")).toBeInTheDocument();
    expect(within(first).getByText("You")).toBeInTheDocument();
    expect(within(second).getByText("Agent")).toBeInTheDocument();
    // A short first page has no next page.
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("pages by the last id and stops at a short page", async () => {
    state.data = {
      agentSessionAttachments: Array.from({ length: FILES_PAGE_SIZE }, (_, i) =>
        row(i),
      ),
    };
    fetchMore.mockResolvedValue({
      data: { agentSessionAttachments: [row(99)] },
    });
    render(<SessionFilesTable sessionId="ags-1" />);
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(fetchMore).toHaveBeenCalledTimes(1));
    expect(fetchMore.mock.calls[0][0].variables).toEqual({
      cursor: `att-${FILES_PAGE_SIZE - 1}`,
    });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Load more" })).toBeNull(),
    );
  });

  it("says so when there are no files, or the list fails", () => {
    state.data = { agentSessionAttachments: [] };
    const { unmount } = render(<SessionFilesTable sessionId="ags-1" />);
    expect(screen.getByText(/No files yet/)).toBeInTheDocument();
    unmount();
    state.data = undefined;
    state.error = new Error("boom");
    render(<SessionFilesTable sessionId="ags-1" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load");
  });

  it("downloads through a freshly signed URL, not the listed one", async () => {
    state.data = { agentSessionAttachments: [row(1)] };
    const assign = vi.fn();
    Object.defineProperty(window, "location", {
      value: { assign },
      writable: true,
    });
    query.mockResolvedValue({
      data: {
        agentAttachment: { downloadUrl: "https://storage.example.test/fresh" },
      },
    });
    render(<SessionFilesTable sessionId="ags-1" />);
    fireEvent.click(screen.getByRole("button", { name: /file-1\.txt/ }));
    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith("https://storage.example.test/fresh"),
    );
    expect(query.mock.calls[0][0]).toMatchObject({
      variables: { id: "att-1" },
      fetchPolicy: "network-only",
    });
  });
});

describe("SessionFilesTable load more", () => {
  it("recovers from a failed page with an error and a retry", async () => {
    state.data = {
      agentSessionAttachments: Array.from({ length: FILES_PAGE_SIZE }, (_, i) =>
        row(i),
      ),
    };
    fetchMore.mockRejectedValueOnce(new Error("network"));
    render(<SessionFilesTable sessionId="ags-1" />);
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not load",
    );
    // The button is back for a retry, not stuck loading.
    expect(screen.getByRole("button", { name: "Load more" })).toBeEnabled();
  });

  it("asks for one fresh URL even when clicked twice", async () => {
    state.data = { agentSessionAttachments: [row(1)] };
    const assign = vi.fn();
    Object.defineProperty(window, "location", {
      value: { assign },
      writable: true,
    });
    let resolve: (v: unknown) => void = () => {};
    query.mockReturnValue(new Promise((r) => (resolve = r)));
    render(<SessionFilesTable sessionId="ags-1" />);
    const button = screen.getByRole("button", { name: /file-1\.txt/ });
    fireEvent.click(button);
    fireEvent.click(button);
    resolve({
      data: {
        agentAttachment: { downloadUrl: "https://storage.example.test/fresh" },
      },
    });
    await waitFor(() => expect(assign).toHaveBeenCalledTimes(1));
    expect(query).toHaveBeenCalledTimes(1);
  });
});
