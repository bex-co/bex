import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import i18n from "@/i18n/init";
import { ServiceEventFilter } from "@/features/events/components/service-event-filter";
import { SERVICE_EVENT_GROUPS } from "@/features/events/service-event-catalog";

function Harness({ extraTypes }: { extraTypes?: string[] }) {
  // Mirrors the route: the filter starts with nothing hidden, so "all types"
  // means whatever the feed carries rather than whatever the catalog lists.
  const [hidden, setHidden] = useState(() => new Set<string>());
  return (
    <ServiceEventFilter
      hidden={hidden}
      onChange={setHidden}
      extraTypes={extraTypes}
    />
  );
}

describe("ServiceEventFilter", () => {
  it("exposes the truthful Render-shaped groups and filters them by search", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));

    expect(screen.getByRole("checkbox", { name: "Deploy" })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Service Status" }),
    ).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Scaling" })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Maintenance Mode" }),
    ).toBeChecked();
    expect(
      screen.queryByRole("checkbox", { name: "Pipeline Minutes Exhausted" }),
    ).not.toBeInTheDocument();

    await user.type(
      screen.getByRole("textbox", { name: "Search events" }),
      "commit ignored",
    );
    expect(
      screen.getByRole("checkbox", { name: "Commit ignored" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "Image pull failed" }),
    ).not.toBeInTheDocument();
  });

  it("supports group selection and reports the narrowed selected count", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));
    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    await user.click(screen.getByRole("checkbox", { name: "Deploy" }));

    // The Deploy group carries the full deploy lifecycle (w7/m66 added
    // build/pre-deploy/job-run-ended): 12 types.
    expect(
      screen.getByRole("button", { name: /Filter events \(12\)/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Deploy started" }),
    ).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Resumed" })).not.toBeChecked();
  });

  it("offers the five types that drifted out of the catalog (w6/m122)", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));

    // Live capture on 2026-08-27 returned exactly three domain options and zero
    // disk options; these five were emitted by the API and unselectable.
    for (const label of [
      "Custom domain verified",
      "Disk created",
      "Disk updated",
      "Disk deleted",
      "Disk restored",
    ]) {
      expect(screen.getByRole("checkbox", { name: label })).toBeChecked();
    }
  });

  it("lists a type the catalog does not know, under Other and pre-selected", async () => {
    const user = userEvent.setup();
    render(<Harness extraTypes={["a_future_backend_type"]} />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));

    // Uncatalogued types are still controllable — the catalog governs grouping
    // and naming, not visibility. The option is labelled by its raw wire type:
    // the generic fallback is already ip_allow_list_changed's own label, so
    // reusing it here would produce two identical, unidentifiable checkboxes.
    const option = screen.getByRole("checkbox", {
      name: "a_future_backend_type",
    });
    expect(option).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Other" })).toBeChecked();

    await user.click(option);
    expect(option).not.toBeChecked();
  });

  it("select-all re-admits an uncatalogued type instead of re-hiding it", async () => {
    const user = userEvent.setup();
    render(<Harness extraTypes={["a_future_backend_type"]} />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));

    // Hide everything, then select all: the old catalog-derived select-all
    // rebuilt the set from SERVICE_EVENT_TYPES and silently dropped anything the
    // catalog had never heard of.
    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    expect(
      screen.getByRole("checkbox", { name: "a_future_backend_type" }),
    ).not.toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    expect(
      screen.getByRole("checkbox", { name: "a_future_backend_type" }),
    ).toBeChecked();
    expect(
      screen.getByRole("button", { name: "Filter events" }),
    ).toBeInTheDocument();
  });
});

describe("ServiceEventFilter — every option has its own name (w4/223)", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  async function optionNames() {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(
      screen.getByRole("button", { name: /^Filter events|^筛选/ }),
    );
    const groupNames = new Set([
      i18n.t("services.eventsFilterAll"),
      ...SERVICE_EVENT_GROUPS.map((g) =>
        i18n.t(`services.eventsFilterGroup.${g.key}`),
      ),
    ]);
    return screen
      .getAllByRole("checkbox")
      .map((box) => box.getAttribute("aria-label") ?? "")
      .filter((name) => name !== "" && !groupNames.has(name));
  }

  it.each(["en", "zh"])(
    "gives each catalogued type a distinct accessible name (%s)",
    async (lang) => {
      await i18n.changeLanguage(lang);
      const names = await optionNames();
      const typeCount = SERVICE_EVENT_GROUPS.reduce(
        (n, g) => n + g.types.length,
        0,
      );
      expect(names).toHaveLength(typeCount);
      expect(new Set(names).size).toBe(names.length);
    },
  );

  it("selects the shutdown option alone by its specific name", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));
    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    await user.click(
      screen.getByRole("checkbox", { name: "Max shutdown delay changed" }),
    );

    expect(
      screen.getByRole("button", { name: /Filter events \(1\)/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Root directory changed" }),
    ).not.toBeChecked();
    expect(
      screen.queryByRole("checkbox", {
        name: "Build and deploy settings changed",
      }),
    ).not.toBeInTheDocument();
  });

  it("finds an option by its specific label and by its wire type", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));
    const search = screen.getByRole("textbox", { name: "Search events" });

    await user.type(search, "shutdown");
    expect(
      screen.getByRole("checkbox", { name: "Max shutdown delay changed" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "Root directory changed" }),
    ).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "auto_deploy_disabled");
    expect(
      screen.getByRole("checkbox", { name: "Auto-deploy disabled" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "Auto-deploy enabled" }),
    ).not.toBeInTheDocument();
  });
});
