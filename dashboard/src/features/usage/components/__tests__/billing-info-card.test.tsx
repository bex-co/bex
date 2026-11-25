// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BillingInfoCardView } from "@/features/usage/components/billing-info-card";
import { BillingInfoCardSkeleton } from "@/features/usage/components/billing-info-card-skeleton";
import {
  useBillingInfo,
  type BillingInfo,
  type SaveBillingInfoResult,
} from "@/features/usage/hooks/use-billing-info";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

vi.mock("@/features/usage/hooks/use-billing-info", () => ({
  useBillingInfo: vi.fn(),
}));

const mocked = vi.mocked(useBillingInfo);
const NOT_READY = "Add a payment method first; billing info is kept with it.";

const UNSET: BillingInfo = {
  __typename: "WorkspaceBillingInfo",
  workspaceId: "tea-a",
  customerReady: true,
  billTo: "acme",
  companyName: "",
  address: null,
  taxId: null,
};

const SET: BillingInfo = {
  ...UNSET,
  billTo: "Acme GmbH",
  companyName: "Acme GmbH",
  address: {
    __typename: "BillingAddress",
    line1: "Hauptstr. 1",
    line2: "",
    city: "Berlin",
    state: "",
    postalCode: "10115",
    country: "DE",
  },
  taxId: { __typename: "BillingTaxId", type: "eu_vat", value: "DE123456789" },
};

function billingInfo(
  info: BillingInfo,
  outcome: SaveBillingInfoResult = { ok: true },
) {
  const save = vi.fn().mockResolvedValue(outcome);
  mocked.mockReturnValue({
    billingInfo: info,
    loading: false,
    error: undefined,
    saving: false,
    save,
  });
  return save;
}

const row = (name: string) =>
  document.querySelector<HTMLElement>(`[data-billing-info-row="${name}"] dd`)!
    .textContent;

async function openDialog(name: "Add billing info" | "Edit") {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name }));
  return { user, dialog: await screen.findByRole("dialog") };
}

async function chooseCountry(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) {
  await user.click(screen.getByRole("combobox", { name: "Country" }));
  await user.click(await screen.findByRole("option", { name }));
}

// w6/m628: as on Render, a workspace adds billing info (company name, address,
// VAT or tax ID) to be shown on all invoices.
describe("BillingInfoCard", () => {
  beforeEach(() => {
    mocked.mockReset();
  });

  it("reads each unset part as not set, billing the workspace's name", () => {
    billingInfo(UNSET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    expect(row("bill-to")).toBe("acme");
    expect(row("company-name")).toBe("Not set");
    expect(row("address")).toBe("Not set");
    expect(row("tax-id")).toBe("Not set");
    expect(
      screen.getByRole("button", { name: "Add billing info" }),
    ).toBeEnabled();
    expect(screen.queryByText(NOT_READY)).not.toBeInTheDocument();
  });

  it("shows what invoices carry: the address on lines, the country and tax ID type by name", () => {
    billingInfo(SET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    expect(row("bill-to")).toBe("Acme GmbH");
    expect(row("company-name")).toBe("Acme GmbH");
    const address = document.querySelector(
      '[data-billing-info-row="address"] dd',
    )!;
    expect([...address.children].map((line) => line.textContent)).toEqual([
      "Hauptstr. 1",
      "Berlin, 10115",
      "Germany",
    ]);
    expect(row("tax-id")).toBe("EU VAT DE123456789");
    expect(screen.getByRole("button", { name: "Edit" })).toBeEnabled();
  });

  it("shows the tax ID field once a country with tax ID types is chosen", async () => {
    billingInfo(UNSET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    const { user, dialog } = await openDialog("Add billing info");
    const scope = within(dialog);
    expect(scope.queryByLabelText("Tax ID")).not.toBeInTheDocument();
    expect(scope.queryByLabelText("EU VAT")).not.toBeInTheDocument();

    // One type: no picker, the field is named by it, its example shown.
    await chooseCountry(user, "France");
    expect(scope.getByLabelText("EU VAT")).toHaveAttribute(
      "placeholder",
      "FRAB123456789",
    );
    expect(
      scope.queryByRole("combobox", { name: "Tax ID type" }),
    ).not.toBeInTheDocument();

    // Several: a picker, its default first.
    await chooseCountry(user, "Germany");
    const picker = scope.getByRole("combobox", { name: "Tax ID type" });
    expect(picker).toHaveTextContent("EU VAT");
    expect(scope.getByLabelText("Tax ID")).toHaveAttribute(
      "placeholder",
      "DE123456789",
    );
    await user.click(picker);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["EU VAT", "DE STN"]);
    await user.click(screen.getByRole("option", { name: "DE STN" }));
    expect(scope.getByLabelText("Tax ID")).toHaveAttribute(
      "placeholder",
      "1234567890",
    );

    // A country Stripe has no type for has no field.
    await chooseCountry(user, "Antarctica");
    expect(scope.queryByLabelText("Tax ID")).not.toBeInTheDocument();
    expect(
      scope.queryByRole("combobox", { name: "Tax ID type" }),
    ).not.toBeInTheDocument();
  });

  it("sends the whole billing info, trimmed, and closes", async () => {
    const save = billingInfo(UNSET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    const { user, dialog } = await openDialog("Add billing info");
    const scope = within(dialog);
    await user.type(scope.getByLabelText("Company name"), " Acme GmbH ");
    await chooseCountry(user, "Germany");
    await user.type(scope.getByLabelText("Address line 1"), "Hauptstr. 1");
    await user.type(scope.getByLabelText("City"), "Berlin");
    await user.type(scope.getByLabelText("Postal code"), "10115");
    await user.type(scope.getByLabelText("Tax ID"), " DE123456789 ");
    await user.click(scope.getByRole("button", { name: "Save" }));
    expect(save).toHaveBeenCalledWith({
      companyName: "Acme GmbH",
      address: {
        line1: "Hauptstr. 1",
        line2: "",
        city: "Berlin",
        state: "",
        postalCode: "10115",
        country: "DE",
      },
      taxId: { type: "eu_vat", value: "DE123456789" },
    });
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("removes what is cleared: no company name, no address, no tax ID", async () => {
    const save = billingInfo(SET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    const { user, dialog } = await openDialog("Edit");
    const scope = within(dialog);
    expect(scope.getByLabelText("Company name")).toHaveValue("Acme GmbH");
    expect(scope.getByRole("combobox", { name: "Country" })).toHaveTextContent(
      "Germany",
    );
    expect(scope.getByLabelText("Tax ID")).toHaveValue("DE123456789");
    await user.clear(scope.getByLabelText("Company name"));
    for (const label of ["Address line 1", "City", "Postal code", "Tax ID"]) {
      await user.clear(scope.getByLabelText(label));
    }
    await chooseCountry(user, "No country");
    await user.click(scope.getByRole("button", { name: "Save" }));
    expect(save).toHaveBeenCalledWith({
      companyName: "",
      address: null,
      taxId: null,
    });
  });

  it("asks for a country before an address line, and sends nothing", async () => {
    const save = billingInfo(UNSET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    const { user, dialog } = await openDialog("Add billing info");
    const scope = within(dialog);
    await user.type(scope.getByLabelText("City"), "Berlin");
    await user.click(scope.getByRole("button", { name: "Save" }));
    expect(save).not.toHaveBeenCalled();
    const country = scope.getByRole("combobox", { name: "Country" });
    expect(country).toHaveAttribute("aria-invalid", "true");
    expect(country).toHaveAccessibleDescription("Choose a country");
  });

  it("shows a field the server refused under that field, and stays open", async () => {
    const save = billingInfo(UNSET, {
      ok: false,
      fieldError: {
        field: "taxId.value",
        message: "Invalid value for eu_vat.",
      },
    });
    render(<BillingInfoCardView workspaceId="tea-a" />);
    const { user, dialog } = await openDialog("Add billing info");
    const scope = within(dialog);
    await chooseCountry(user, "France");
    await user.type(scope.getByLabelText("EU VAT"), "123");
    await user.click(scope.getByRole("button", { name: "Save" }));
    expect(save).toHaveBeenCalledTimes(1);
    const value = await scope.findByLabelText("EU VAT");
    await waitFor(() =>
      expect(value).toHaveAccessibleDescription("Invalid value for eu_vat."),
    );
    expect(value).toHaveAttribute("aria-invalid", "true");
    expect(value).toHaveValue("123");
    // Editing the field drops the refusal.
    await user.type(value, "4");
    expect(value).not.toHaveAccessibleDescription("Invalid value for eu_vat.");
  });

  it("keeps a stored tax ID type the country does not list choosable", async () => {
    billingInfo({
      ...SET,
      address: { ...SET.address!, country: "US" },
    });
    render(<BillingInfoCardView workspaceId="tea-a" />);
    const { user, dialog } = await openDialog("Edit");
    const picker = within(dialog).getByRole("combobox", {
      name: "Tax ID type",
    });
    expect(picker).toHaveTextContent("EU VAT");
    await user.click(picker);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["US EIN", "EU VAT"]);
  });

  it("holds the action, with a note, while there is no billing account", () => {
    const save = billingInfo({ ...UNSET, customerReady: false });
    render(<BillingInfoCardView workspaceId="tea-a" />);
    expect(screen.getByText(NOT_READY)).toBeInTheDocument();
    const add = screen.getByRole("button", { name: "Add billing info" });
    expect(add).toBeDisabled();
    expect(save).not.toHaveBeenCalled();
  });

  it("does not pass a failed read off as billing info, or as none", () => {
    mocked.mockReturnValue({
      billingInfo: null,
      loading: false,
      error: new Error("network down"),
      saving: false,
      save: vi.fn(),
    });
    render(<BillingInfoCardView workspaceId="tea-a" />);
    expect(
      screen.getByText("The billing info could not be loaded."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Not set")).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("shows its skeleton while the first read is in flight", () => {
    mocked.mockReturnValue({
      billingInfo: null,
      loading: true,
      error: undefined,
      saving: false,
      save: vi.fn(),
    });
    render(<BillingInfoCardView workspaceId="tea-a" />);
    expect(screen.getByTestId("billing-info-skeleton")).toBeInTheDocument();
  });
});

// w9/m620: a Developer views billing details but does not change them.
describe("BillingInfoCard for a member who views but does not manage billing", () => {
  it("shows the billing info with its action held", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManageBilling: false }),
    );
    const save = billingInfo(SET);
    render(<BillingInfoCardView workspaceId="tea-a" />);
    expect(row("company-name")).toBe("Acme GmbH");
    const edit = screen.getByRole("button", { name: "Edit" });
    expect(edit).toBeDisabled();
    await userEvent.setup().click(edit);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });
});

// The pending card keeps the ready card's header, its four labelled rows and
// the action a workspace with no billing info sees (dashboard/AGENTS.md).
describe("BillingInfoCard skeleton", () => {
  it("matches the ready card's structure, rows and action", () => {
    billingInfo(UNSET);
    const ready = render(<BillingInfoCardView workspaceId="tea-a" />);
    const pending = render(<BillingInfoCardSkeleton />);
    // The card's own regions, less the bars that stand in for data and the
    // button a reserved bar of its own box stands in for.
    const shape = (root: HTMLElement) =>
      Array.from(root.querySelectorAll("[data-slot]"))
        .map((el) => el.getAttribute("data-slot"))
        .filter((slot) => slot !== "skeleton" && slot !== "button");
    expect(shape(pending.container)).toEqual(shape(ready.container));
    // The same rows, labelled alike, each one line tall in both.
    const rows = (root: HTMLElement) =>
      Array.from(root.querySelectorAll("[data-billing-info-row]")).map((el) => [
        el.getAttribute("data-billing-info-row"),
        el.querySelector("dt")!.textContent,
        el.querySelector("dd")!.className,
      ]);
    expect(rows(pending.container)).toEqual(rows(ready.container));
    expect(rows(ready.container).map(([, label]) => label)).toEqual([
      "Bill to",
      "Company name",
      "Address",
      "Tax ID",
    ]);
    // The action's label and box, and no data or resolved copy.
    const content = (root: HTMLElement) =>
      root.querySelector('[data-slot="card-content"]')!;
    expect(content(pending.container).textContent).toContain(
      "Add billing info",
    );
    expect(content(pending.container).textContent).not.toContain("Not set");
    expect(content(pending.container).textContent).not.toContain("acme");
    expect(content(pending.container).querySelector("button")).toBeNull();
    expect(
      content(pending.container).querySelector(
        "span.relative.inline-flex > span.invisible",
      )?.className,
    ).toContain("h-8");
    expect(
      content(ready.container).querySelector("button")?.className,
    ).toContain("h-8");
  });
});
