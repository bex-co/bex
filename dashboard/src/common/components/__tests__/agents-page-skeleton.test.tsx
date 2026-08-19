import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AgentsPageSkeleton } from "@/common/components/detail-skeletons";

describe("AgentsPageSkeleton", () => {
  it("is a composer+recents placeholder, not the 3-column list card grid", () => {
    const { container } = render(<AgentsPageSkeleton />);
    expect(screen.getByTestId("agents-page-skeleton")).toBeInTheDocument();
    expect(container.querySelector(".lg\\:grid-cols-3")).toBeNull();
  });
});
