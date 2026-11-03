import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { TurnAttachments } from "@/features/agent-sessions/components/turn-attachments";

const refetch = vi.fn();
const query = vi.fn();
const queryState: { downloadUrl: string | undefined; loading: boolean } = {
  downloadUrl: "https://storage.example.test/a?sig=1",
  loading: false,
};
vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: queryState.downloadUrl
      ? { agentAttachment: { downloadUrl: queryState.downloadUrl } }
      : undefined,
    loading: queryState.loading,
    refetch,
  }),
  useApolloClient: () => ({ query }),
}));

const originalLocation = window.location;
beforeEach(() => {
  refetch.mockReset();
  query.mockReset();
  queryState.downloadUrl = "https://storage.example.test/a?sig=1";
  queryState.loading = false;
});
afterEach(() => {
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
  });
});

const png = {
  id: "att-1",
  name: "shot.png",
  mediaType: "image/png",
  size: 2048,
};
const svg = {
  id: "att-2",
  name: "logo.svg",
  mediaType: "image/svg+xml",
  size: 300,
};
const log = {
  id: "att-3",
  name: "crash.log",
  mediaType: "text/plain",
  size: 12,
};

describe("TurnAttachments", () => {
  it("renders raster images as thumbnails and everything else as cards", () => {
    render(<TurnAttachments items={[png, svg, log]} />);
    const img = screen.getByAltText("shot.png");
    expect(img).toHaveAttribute("src", "https://storage.example.test/a?sig=1");
    // SVG is never put in an <img> or inlined: it is a download card.
    expect(screen.queryByAltText("logo.svg")).toBeNull();
    expect(
      screen.getByRole("button", { name: "Download logo.svg" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Download crash.log" }),
    ).toBeInTheDocument();
  });

  it("renews an expired link once per link, then shows it as unavailable", () => {
    const { rerender } = render(<TurnAttachments items={[png]} />);
    fireEvent.error(screen.getByAltText("shot.png"));
    expect(refetch).toHaveBeenCalledTimes(1);

    // The renewed link loads, and later expires too: it gets its own renewal.
    queryState.downloadUrl = "https://storage.example.test/a?sig=2";
    rerender(<TurnAttachments items={[png]} />);
    expect(screen.getByAltText("shot.png")).toHaveAttribute(
      "src",
      "https://storage.example.test/a?sig=2",
    );
    fireEvent.error(screen.getByAltText("shot.png"));
    expect(refetch).toHaveBeenCalledTimes(2);

    // A link that fails again after its own renewal is really gone.
    queryState.downloadUrl = "https://storage.example.test/a?sig=3";
    rerender(<TurnAttachments items={[png]} />);
    fireEvent.error(screen.getByAltText("shot.png"));
    expect(refetch).toHaveBeenCalledTimes(3);
    fireEvent.error(screen.getByAltText("shot.png"));
    expect(refetch).toHaveBeenCalledTimes(3);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("shows an unavailable card when the link query settles empty", () => {
    queryState.downloadUrl = undefined;
    render(<TurnAttachments items={[png]} />);
    expect(screen.queryByAltText("shot.png")).toBeNull();
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("fetches a fresh link when a card is clicked", async () => {
    query.mockResolvedValue({
      data: {
        agentAttachment: { downloadUrl: "https://storage.example.test/fresh" },
      },
    });
    const assign = vi.fn();
    Object.defineProperty(window, "location", {
      value: { ...originalLocation, assign },
      writable: true,
    });
    render(<TurnAttachments items={[log]} />);
    fireEvent.click(screen.getByRole("button", { name: "Download crash.log" }));
    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith("https://storage.example.test/fresh"),
    );
    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({
        fetchPolicy: "network-only",
        variables: { id: "att-3" },
      }),
    );
  });

  it("renders nothing for a turn without attachments", () => {
    const { container } = render(<TurnAttachments items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
