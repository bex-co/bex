import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EventIcon } from "../event-icon";

describe("EventIcon", () => {
  it.each([
    ["deploy_started", "rocket"],
    ["build_started", "hammer"],
    ["pre_deploy_started", "terminal"],
    ["branch_deleted", "git-branch"],
    ["image_pull_failed", "circle-x"],
    ["server_failed", "circle-x"],
    ["suspender_added", "circle-pause"],
    ["service_suspended", "circle-pause"],
    ["service_hibernated", "moon"],
    ["service_woken", "sunrise"],
    ["suspender_removed", "circle-play"],
    ["service_resumed", "circle-play"],
    ["server_available", "circle-play"],
    ["server_restarted", "refresh-ccw"],
    ["custom_domain_verified", "globe"],
    ["service_moved", "folder-input"],
    ["disk_created", "hard-drive"],
    ["disk_updated", "hard-drive"],
    ["disk_deleted", "unplug"],
    ["disk_restored", "history"],
    ["instance_count_changed", "scale"],
    ["autoscaling_config_changed", "scale"],
    ["autoscaling_started", "scale"],
    ["autoscaling_ended", "scale"],
    ["unknown_event", "circle-dot"],
    ["constructor", "circle-dot"],
  ])("shows %s independently of unrelated outcome fields", (type, icon) => {
    const { container } = render(
      <EventIcon type={type} status="update_failed" factStatus="failed" />,
    );
    expect(container.querySelector("svg")).toHaveClass(
      `lucide-${icon}`,
      "size-4",
    );
    expect(container.querySelector("svg")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it.each(["build_ended", "pre_deploy_ended", "job_run_ended"])(
    "uses the lifecycle outcome for %s",
    (type) => {
      const { container, rerender } = render(
        <EventIcon type={type} status="update_failed" factStatus="failed" />,
      );
      expect(container.querySelector("svg")).toHaveClass("lucide-circle-x");
      rerender(
        <EventIcon type={type} status="update_failed" factStatus="canceled" />,
      );
      expect(container.querySelector("svg")).toHaveClass("lucide-ban");
      rerender(<EventIcon type={type} status="update_failed" factStatus="" />);
      expect(container.querySelector("svg")).toHaveClass("lucide-circle-check");
    },
  );

  it("uses deploy status for deploy endings, including its success fallback", () => {
    const { container, rerender } = render(
      <EventIcon
        type="deploy_ended"
        status="update_failed"
        factStatus="succeeded"
      />,
    );
    expect(container.querySelector("svg")).toHaveClass("lucide-circle-x");
    rerender(<EventIcon type="deploy_ended" status="" factStatus="failed" />);
    expect(container.querySelector("svg")).toHaveClass("lucide-circle-check");
  });
});
