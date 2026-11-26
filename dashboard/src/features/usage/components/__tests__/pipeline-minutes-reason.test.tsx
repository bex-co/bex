import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import i18n, { ensureLanguage } from "@/i18n/init";
import { SUPPORTED_LANGUAGES } from "@/i18n/config";
import {
  PIPELINE_MINUTES_DOCS_URL,
  PipelineMinutesReason,
} from "@/features/usage/components/pipeline-minutes-reason";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    hash,
    children,
  }: {
    to: string;
    hash?: string;
    children: React.ReactNode;
  }) => <a href={`${to}#${hash}`}>{children}</a>,
}));

// Render's notice (`Fk`, w3/108): "You’ve used up the maximum number of
// pipeline minutes specified by your spend limit." with the minutes linking
// Workspace Settings → Build Pipeline, then "Learn more." on its own line.
describe("PipelineMinutesReason", () => {
  it.each(SUPPORTED_LANGUAGES)(
    "puts each link where %s's sentence names it, the docs link on its own line",
    async (lng) => {
      await ensureLanguage(lng);
      await i18n.changeLanguage(lng);
      try {
        const t = i18n.getFixedT(lng);
        const { container } = render(<PipelineMinutesReason />);
        const minutes = screen.getByRole("link", {
          name: t("usage.pipelineMinutesLink"),
        });
        const docs = screen.getByRole("link", {
          name: t("usage.pipelineMinutesLearnMore"),
        });
        expect(minutes).toHaveAttribute(
          "href",
          "/workspace/settings#build-pipeline",
        );
        expect(docs).toHaveAttribute("href", PIPELINE_MINUTES_DOCS_URL);
        expect(docs).toHaveAttribute("target", "_blank");
        // Two lines: the sentence with the minutes link, then the docs link.
        const [sentence, learnMore] = Array.from(
          container.firstElementChild!.children,
        );
        expect(sentence).toContainElement(minutes);
        expect(learnMore).toContainElement(docs);
        // Nothing of the template is left over or lost.
        const whole = t("usage.pipelineMinutesExceeded", {
          minutes: t("usage.pipelineMinutesLink"),
          learnMore: t("usage.pipelineMinutesLearnMore"),
        });
        expect(container.textContent!.replace(/\s/g, "")).toBe(
          whole.replace(/\s/g, ""),
        );
      } finally {
        await i18n.changeLanguage("en");
      }
    },
  );

  it("reads Render's words in English", () => {
    const { container } = render(<PipelineMinutesReason />);
    expect(container).toHaveTextContent(
      "You’ve used up the maximum number of pipeline minutes specified by your spend limit.Learn more.",
    );
  });
});
