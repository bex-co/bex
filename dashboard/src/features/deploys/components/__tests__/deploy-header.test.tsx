import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeployHeader } from "@/features/deploys/components/deploy-header";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    children,
    ...rest
  }: {
    to: string;
    params?: Record<string, string>;
    children?: React.ReactNode;
    [key: string]: unknown;
  }) => {
    const href = Object.entries(params ?? {}).reduce(
      (acc, [key, value]) => acc.replace(`$${key}`, value),
      to,
    );
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  },
}));

function deploy(over: Partial<DeployView> = {}): DeployView {
  return {
    id: "dep-1",
    status: "live",
    trigger: "api",
    image: "registry.example.com/web:1",
    imageSha: "",
    rollbackOf: "",
    buildAvailable: null,
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-07-14T00:00:00Z",
    updatedAt: "2026-07-14T00:01:00Z",
    startedAt: "2026-07-14T00:00:01Z",
    finishedAt: "2026-07-14T00:01:00Z",
    preDeployStatus: "",
    initialDeployHookStatus: "",
    initialDeployHookFinishedAt: null,
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    failureReasonTemplate: null,
    stallReasonTemplate: null,
    cancelReasonTemplate: null,
    ...over,
  };
}

describe("DeployHeader", () => {
  it("shows the digest a live image deploy ran beside its image, with copy (w8/m122)", () => {
    const sha =
      "sha256:4f1e2a9b8c7d6e5f40312233445566778899aabbccddeeff0011223344556677";
    render(
      <DeployHeader
        deploy={deploy({
          image: "docker.io/ealen/echo-server:0.9.2",
          imageSha: sha,
        })}
      />,
    );
    expect(
      screen.getByText("docker.io/ealen/echo-server:0.9.2"),
    ).toBeInTheDocument();
    const short = screen.getByText("sha256:4f1e2a9b8c7d");
    expect(short).toHaveAttribute("title", sha);
    expect(
      screen.getByRole("button", { name: "Copy image digest" }),
    ).toBeInTheDocument();
  });

  it("omits the digest while it is unknown", () => {
    render(<DeployHeader deploy={deploy({ imageSha: "" })} />);
    expect(screen.queryByText(/^sha256:/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy image digest" }),
    ).not.toBeInTheDocument();
  });

  it.each([
    ["live", "Live"],
    ["update_in_progress", "In Progress"],
    ["update_failed", "Failed"],
    ["canceled", "Canceled"],
    ["build_failed", "Build Failed"],
    ["pre_deploy_failed", "Pre-Deploy Failed"],
    ["deactivated", "Deactivated"],
  ])("renders the %s status as %j", (status, label) => {
    render(<DeployHeader deploy={deploy({ status })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it.each([
    ["running", "Running"],
    ["succeeded", "Succeeded"],
    ["failed", "Failed"],
    ["canceled", "Canceled"],
  ])("renders the %s pre-deploy outcome", (preDeployStatus, label) => {
    // A status the deploy itself cannot share with this word, so the match is
    // the Pre-deploy fact's.
    render(
      <DeployHeader deploy={deploy({ status: "queued", preDeployStatus })} />,
    );
    const fact = screen.getByText("Pre-deploy").closest("div");
    expect(fact).toHaveTextContent(label);
  });

  it("shows no pre-deploy fact when the deploy has no pre-deploy step", () => {
    render(<DeployHeader deploy={deploy({ preDeployStatus: "" })} />);
    expect(screen.queryByText("Pre-deploy")).not.toBeInTheDocument();
  });

  it.each([
    ["api", "API Deploy"],
    ["manual", "Manual Deploy"],
  ])("labels a %s-triggered deploy as %s", (trigger, label) => {
    render(<DeployHeader deploy={deploy({ trigger, rollbackOf: "" })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("labels a rollback deploy with the restored deploy's id intact (no CSS capitalize)", () => {
    render(
      <DeployHeader
        deploy={deploy({ trigger: "rollback", rollbackOf: "dep-live-001" })}
      />,
    );
    const label = screen.getByText("Rollback to dep-live-001");
    expect(label).toBeInTheDocument();
    expect(label).not.toHaveClass("capitalize");
  });

  // Render titles the deploy page with the commit's subject.
  it("titles the deploy with the commit's first line and shows its short SHA (w9/001)", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "abc1234def5678",
          commitMessage: "fix: header\n\nlonger body that must not render",
        })}
      />,
    );
    expect(
      screen.getByRole("heading", { level: 2, name: "fix: header" }),
    ).toBeInTheDocument();
    expect(screen.getByText("abc1234")).toBeInTheDocument();
    expect(screen.queryByText(/longer body/)).not.toBeInTheDocument();
    // The deploy's own id stays on the page, copyable.
    expect(screen.getByText("dep-1")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy Deploy ID" }),
    ).toBeInTheDocument();
  });

  it("links the short SHA to its diff when the repo is browsable", () => {
    render(
      <DeployHeader
        repo="https://github.com/acme/web"
        deploy={deploy({ commitId: "abc1234def5678", commitMessage: "fix" })}
      />,
    );
    expect(screen.getByRole("link", { name: "abc1234" })).toHaveAttribute(
      "href",
      "https://github.com/acme/web/commit/abc1234def5678",
    );
  });

  // Hash-only commits (message unavailable) must still render cleanly — the
  // public-repo resolve path can yield SHA before a subject is known.
  it("renders a hash-only commit without a dangling message spacer", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "abc1234def5678",
          commitMessage: "",
        })}
      />,
    );
    // No subject to title it by: the deploy's id is its name.
    expect(
      screen.getByRole("heading", { level: 2, name: "dep-1" }),
    ).toBeInTheDocument();
    const line = screen.getByText("abc1234").parentElement;
    expect(line?.textContent?.trim()).toBe("abc1234");
  });

  // w6/m45 t004: the date used to be separated from the commit by a margin
  // alone, so the line was one unbroken text run — a screen reader (and anyone
  // copying the line) got "…Docker build contextAugust 22, 2026".
  it("separates the commit from the commit date with real text", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "abc1234def5678",
          commitMessage: "fix: Docker build context",
          commitCreatedAt: "2026-07-14T00:00:00Z",
        })}
      />,
    );
    const line = screen.getByText("abc1234").parentElement;
    expect(line?.textContent).toMatch(/abc1234\s+·\s+\S/);
  });

  it("shows no commit line when the commit was never resolved", () => {
    render(<DeployHeader deploy={deploy({ commitId: "" })} />);
    expect(screen.queryByText("abc1234")).not.toBeInTheDocument();
  });

  it("renders the deploy's image", () => {
    render(
      <DeployHeader
        deploy={deploy({ image: "registry.example.com/web:abc123" })}
      />,
    );
    // Whole, with a break opportunity after each slash rather than a cut.
    const image = screen.getByText("registry.example.com/web:abc123");
    expect(image.querySelectorAll("wbr")).toHaveLength(1);
  });

  it("shows duration through the shared formatter", () => {
    render(<DeployHeader deploy={deploy()} />);

    expect(screen.getByText("Duration")).toBeInTheDocument();
    expect(screen.getByText("59s")).toBeInTheDocument();
  });

  it("omits timestamps and duration that haven't happened instead of inventing values", () => {
    render(
      <DeployHeader
        deploy={deploy({
          status: "update_in_progress",
          createdAt: null,
          updatedAt: null,
          startedAt: null,
          finishedAt: null,
        })}
      />,
    );

    expect(screen.queryByText("Deployed")).not.toBeInTheDocument();
    expect(screen.queryByText("Duration")).not.toBeInTheDocument();
    expect(screen.queryByText("—")).not.toBeInTheDocument();
  });

  // w4/m156: a live deploy whose start was never observed arrives with
  // GraphQL's empty-string encoding. The header shows no Duration — not a 0s
  // span ending at the finish — while keeping the facts that are known.
  it("shows no duration for a finished deploy with an unknown start", () => {
    render(<DeployHeader deploy={deploy({ startedAt: "" })} />);

    expect(screen.queryByText("Duration")).not.toBeInTheDocument();
    expect(screen.queryByText("0s")).not.toBeInTheDocument();
    expect(screen.queryByText("In progress")).not.toBeInTheDocument();
    expect(screen.getByText("Deployed")).toBeInTheDocument();
  });

  it("says a running deploy's duration is still in progress", () => {
    render(
      <DeployHeader
        deploy={deploy({ status: "build_in_progress", finishedAt: null })}
      />,
    );
    expect(screen.getByText("Duration")).toBeInTheDocument();
    expect(screen.getByText("In progress")).toBeInTheDocument();
  });

  // w1/m659: the failure, cancel and stall reasons are the deploy page's
  // Notices now (deploy-notices.test.tsx), so the header leaves them out.
  it("leaves a deploy's reasons to the Notices", () => {
    const { container } = render(
      <DeployHeader
        deploy={deploy({
          status: "update_failed",
          failureReason: "the deploy failed its health check",
          stallReason: "waiting on GET /healthz",
        })}
      />,
    );
    expect(container.textContent).not.toContain("health check");
    expect(container.textContent).not.toContain("/healthz");
  });

  // w2/m612 t005: linked-service deploy headers show buildId and Build Source provenance.
  it("shows copyable buildId and Build Source provenance for linked-service deploys", () => {
    render(
      <DeployHeader
        deploy={deploy({
          buildId: "bld-abc12345",
          commitId: "deadbeef1234",
          commitMessage: "feat: build source rollout",
        })}
        repo="https://github.com/acme/mono"
        buildSourceId="bds-mono001"
        buildSourceName="Monorepo Build"
      />,
    );
    const buildFact = screen.getByTestId("deploy-build-id");
    expect(buildFact).toHaveTextContent("bld-abc12345");
    expect(screen.getByRole("link", { name: "bld-abc12345" })).toHaveAttribute(
      "href",
      "/build-sources/bds-mono001/builds/bld-abc12345",
    );
    expect(
      screen.getByRole("button", { name: "Copy Build ID" }),
    ).toBeInTheDocument();

    const sourceFact = screen.getByTestId("deploy-build-source");
    expect(sourceFact).toHaveTextContent("Monorepo Build");
    expect(
      screen.getByRole("link", { name: "Monorepo Build" }),
    ).toHaveAttribute("href", "/build-sources/bds-mono001");
    expect(sourceFact).toHaveTextContent("bds-mono001");
    expect(
      screen.getByRole("button", { name: "Copy Build Source ID" }),
    ).toBeInTheDocument();
  });

  it("omits Build and Build Source facts for non-linked deploys", () => {
    render(<DeployHeader deploy={deploy({ buildId: "" })} />);
    expect(screen.queryByTestId("deploy-build-id")).not.toBeInTheDocument();
    expect(screen.queryByTestId("deploy-build-source")).not.toBeInTheDocument();
  });

  // w1/m661: Render's Scaling fact, "N of M instances", its numbers set apart.
  it.each([
    [0, 2, "0 of 2 instances"],
    [1, 2, "1 of 2 instances"],
    [2, 2, "2 of 2 instances"],
    [1, 1, "1 of 1 instance"],
  ])("shows Scaling %i of %i as %j", (deployed, total, words) => {
    render(
      <DeployHeader
        deploy={deploy({ status: "update_in_progress" })}
        scaling={{ deployed, total }}
      />,
    );
    const fact = screen.getByText("Scaling").closest("div");
    expect(fact).toHaveTextContent(words);
    const numbers = screen
      .getByTestId("deploy-scaling")
      .querySelectorAll("strong");
    expect([...numbers].map((n) => n.textContent)).toEqual([
      String(deployed),
      String(total),
    ]);
  });

  it("keeps the Scaling fact's place while its count loads", () => {
    render(<DeployHeader deploy={deploy()} scaling="loading" />);
    expect(screen.getByText("Scaling")).toBeInTheDocument();
    expect(screen.queryByTestId("deploy-scaling")).not.toBeInTheDocument();
  });

  it("shows no Scaling fact when the deploy has none", () => {
    render(<DeployHeader deploy={deploy()} scaling={null} />);
    expect(screen.queryByText("Scaling")).not.toBeInTheDocument();
  });

  it("sets Scaling after Source and before Pre-deploy, as Render does", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "deadbeef1234",
          preDeployStatus: "succeeded",
        })}
        scaling={{ deployed: 2, total: 2 }}
      />,
    );
    const labels = [...document.querySelectorAll("dt")].map((dt) =>
      dt.textContent?.trim(),
    );
    const source = labels.indexOf("Source");
    expect(labels.slice(source, source + 3)).toEqual([
      "Source",
      "Scaling",
      "Pre-deploy",
    ]);
  });
});
