import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { GithubIcon } from "../github-icon";

// lucide-react 1.0 dropped its brand icons (w1/m176), so the GitHub mark is
// rebuilt from lucide's factory. It must stay a lucide icon by name, so the
// callers' classes and lucide's shared styling apply to it like any other.
describe("GithubIcon", () => {
  it("renders as lucide's github icon with the caller's classes", () => {
    const { container } = render(<GithubIcon className="size-4" />);
    expect(container.querySelector("svg")).toHaveClass(
      "lucide",
      "lucide-github",
      "size-4",
    );
  });
});
