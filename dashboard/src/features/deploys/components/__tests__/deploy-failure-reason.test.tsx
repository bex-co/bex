import { afterEach, describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import { DeployFailureReason } from "@/features/deploys/components/deploy-failure-reason";

describe("DeployFailureReason", () => {
  it("renders the reason with the destructive treatment by default", () => {
    const { container } = render(
      <DeployFailureReason reason="image pull failed: not found" />,
    );

    const el = screen.getByText("image pull failed: not found");
    expect(el.tagName).toBe("P");
    expect(el.className).toContain("text-destructive");
    expect(container.querySelector("p[title]")).toBeNull();
  });

  it("renders a supersede cancel with the neutral treatment", () => {
    render(
      <DeployFailureReason reason="Superseded by dep-abc" tone="neutral" />,
    );

    const el = screen.getByText("Superseded by dep-abc");
    expect(el.className).toContain("text-muted-foreground");
    expect(el.className).not.toContain("text-destructive");
  });

  it("renders nothing without a reason", () => {
    const { container } = render(<DeployFailureReason reason="" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("truncates with a hover tooltip for list rows", () => {
    render(
      <DeployFailureReason reason="image pull failed: not found" truncate />,
    );

    const el = screen.getByText("image pull failed: not found");
    expect(el.className).toContain("truncate");
    expect(el).toHaveAttribute("title", "image pull failed: not found");
  });

  // w4/566: a reason with a template the dashboard knows reads in its language.
  describe("by template", () => {
    afterEach(async () => {
      await i18n.changeLanguage("en");
    });
    const template = { code: "http_port_detected", params: { port: "80" } };

    it("words a known template in zh, tooltip included", async () => {
      await i18n.changeLanguage("zh");
      render(
        <DeployFailureReason
          reason="Detected HTTP on port 80; serving it"
          template={template}
          truncate
        />,
      );
      const el = screen.getByText(
        "已在端口 80 上检测到 HTTP，将通过该端口提供服务",
      );
      expect(el).toHaveAttribute(
        "title",
        "已在端口 80 上检测到 HTTP，将通过该端口提供服务",
      );
    });

    it("keeps bex-api's sentence for a template it does not know", async () => {
      await i18n.changeLanguage("zh");
      render(
        <DeployFailureReason
          reason="Something new"
          template={{ code: "something_new", params: {} }}
        />,
      );
      expect(screen.getByText("Something new")).toBeInTheDocument();
    });
  });
});
