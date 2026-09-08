import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ServiceOutboundIpsPanel } from "@/features/services/components/service-outbound-ips-panel";

vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({
    t: (key: string, vars?: Record<string, string>) => {
      if (key === "services.outboundIpsCopy" && vars?.ip) {
        return `Copy ${vars.ip}`;
      }
      return key;
    },
  }),
}));

vi.mock("@/common/components/connection-field", () => ({
  ConnectionField: ({ label, value }: { label: string; value: string }) => (
    <div data-testid="connection-field">
      {label}:{value}
    </div>
  ),
}));

describe("ServiceOutboundIpsPanel", () => {
  it("renders an honest empty state when ips is empty", () => {
    render(<ServiceOutboundIpsPanel ips={[]} />);
    expect(screen.getByText("services.outboundIpsEmpty")).toBeInTheDocument();
  });

  it("renders one copy row per IP", () => {
    render(<ServiceOutboundIpsPanel ips={["1.2.3.4", "5.6.7.8"]} />);
    expect(screen.getByText("Copy 1.2.3.4:1.2.3.4")).toBeInTheDocument();
    expect(screen.getByText("Copy 5.6.7.8:5.6.7.8")).toBeInTheDocument();
  });
});
