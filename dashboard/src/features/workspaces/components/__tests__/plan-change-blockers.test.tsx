import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { PlanChangeBlockers } from "@/features/workspaces/components/plan-change-blockers";
import type { PlanChangeBlocker } from "@/features/workspaces/hooks/use-plan-change-blockers";

// Render's plan rules on its plan page (w3/m708), each drawn from one of
// bex-api's refusals (workspacePlanChangeBlockers).
function renderBlockers(
  blockers: PlanChangeBlocker[],
  planName = "Hobby",
  organizationId: string | null = "org-1",
) {
  return render(
    <PlanChangeBlockers
      blockers={blockers}
      planName={planName}
      organizationId={organizationId}
    />,
  );
}

/** Each notice as [title, message, link label, link href]. */
function notices() {
  return screen.getAllByRole("alert").map((alert) => {
    const lines = [...alert.querySelectorAll("p")].map((p) => p.textContent);
    const link = within(alert).queryByRole("link");
    return {
      lines,
      link: link ? [link.textContent, link.getAttribute("href")] : null,
      newTab: link?.getAttribute("target") ?? null,
    };
  });
}

describe("PlanChangeBlockers", () => {
  it("draws nothing when nothing blocks", () => {
    const { container } = renderBlockers([]);
    expect(container).toBeEmptyDOMElement();
  });

  it("draws each rule with Render's title, message and fix link, in Render's order", () => {
    renderBlockers([
      {
        kind: "member_limit",
        items: ["carol@example.com:developer"],
        limit: 1,
      },
      { kind: "service_limit", items: [], limit: 25 },
      { kind: "workspace_limit", items: [], limit: 5 },
      { kind: "secondary_workspaces", items: ["beta", "gamma"], limit: null },
    ]);
    expect(notices()).toEqual([
      {
        lines: [
          "Too many Hobby workspaces.",
          "You cannot have more than 5 Hobby workspaces. Please upgrade or delete another Hobby workspace.",
        ],
        link: [
          "Workspace limits",
          "https://bex.co/docs/platform/members#roles-and-plan-limits",
        ],
        newTab: "_blank",
      },
      {
        lines: [
          "Too many services in workspace.",
          "Hobby workspaces can only have up to 25 services.",
        ],
        link: ["Update services", "/"],
        newTab: "_blank",
      },
      {
        lines: [
          "Too many workspaces.",
          "Hobby doesn’t support more than one workspace.",
          "beta and gamma",
        ],
        link: ["Update workspaces", "/org/org-1/workspaces"],
        newTab: "_blank",
      },
      {
        lines: [
          "Too many users in team.",
          "Hobby workspaces can only have 1 user.",
          "carol@example.com (Developer)",
        ],
        link: ["Update team members", "/workspace/settings#team"],
        newTab: "_blank",
      },
    ]);
  });

  it("says a several-seat cap in Render's plural", () => {
    renderBlockers([{ kind: "member_limit", items: [], limit: 10 }], "Pro");
    expect(
      screen.getByText("Pro workspaces can only have up to 10 users."),
    ).toBeInTheDocument();
  });

  it("is Render's one roles rule for members and pending invites together", () => {
    renderBlockers(
      [
        {
          kind: "member_roles",
          items: ["vic@example.com:viewer"],
          limit: null,
        },
        {
          kind: "invite_roles",
          items: ["bill@example.com:billing", "val@example.com:viewer"],
          limit: null,
        },
      ],
      "Pro",
    );
    expect(notices()).toEqual([
      {
        lines: [
          "Cannot downgrade while using unavailable roles",
          "Pro doesn’t support the Billing, Viewer roles. To downgrade your workspace, first change affected team members and pending invites to Developer or Admin.",
          "vic@example.com (Viewer), bill@example.com (Billing), and val@example.com (Viewer)",
        ],
        link: ["Update team members", "/workspace/settings#team"],
        newTab: "_blank",
      },
    ]);
  });

  it("names one unavailable role in Render's singular", () => {
    renderBlockers(
      [
        {
          kind: "member_roles",
          items: ["vic@example.com:viewer"],
          limit: null,
        },
      ],
      "Pro",
    );
    expect(
      screen.getByText(/^Pro doesn’t support the Viewer role\./),
    ).toBeInTheDocument();
  });

  it("draws bex's inbound IP rules refusal with Render's words for it", () => {
    renderBlockers(
      [
        {
          kind: "inbound_ip_rules",
          items: ["workspace", "environment:production", "service:api"],
          limit: null,
        },
      ],
      "Pro",
    );
    const [notice] = notices();
    expect(notice.lines[0]).toBe(
      "IP allow lists must be removed before downgrading.",
    );
    expect(notice.lines[1]).toBe(
      "Remove all IP allow list configurations from your workspace, environments, and services before changing your plan.",
    );
    expect(notice.lines[2]).toMatch(/production/);
    expect(notice.link).toEqual([
      "Update inbound IP rules",
      "/workspace/settings#inbound-ip-rules",
    ]);
  });

  it("still holds a refusal kind this dashboard has no copy for", () => {
    renderBlockers([{ kind: "something_new", items: [], limit: null }], "Pro");
    expect(notices()).toEqual([
      {
        lines: [
          "This plan can’t be selected yet.",
          "This workspace doesn’t meet the Pro plan’s limits yet.",
        ],
        link: null,
        newTab: null,
      },
    ]);
  });

  it("offers no workspaces link when the org is unknown", () => {
    renderBlockers(
      [{ kind: "secondary_workspaces", items: ["beta"], limit: null }],
      "Pro",
      null,
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
