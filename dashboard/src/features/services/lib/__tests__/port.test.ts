import { describe, expect, it } from "vitest";
import {
  openPortsDisplay,
  settingsPort,
  type PortFacts,
} from "@/features/services/lib/port";

const tcp = (...ports: number[]) =>
  ports.map((port) => ({ port, protocol: "TCP" }));

const privateService = (facts: Partial<PortFacts>): PortFacts => ({
  type: "private_service",
  port: null,
  openPorts: null,
  ...facts,
});

// w6/m148: bex-api's Service.openPorts is null (no network address), [port]
// (configured port), the image's declared TCP ports, or [] while an image-port
// private service has not been read yet.
describe("openPortsDisplay", () => {
  it("lists an image's ports in order, marking only the primary", () => {
    expect(
      openPortsDisplay(
        privateService({ port: 9000, openPorts: tcp(8123, 9000, 9009) }),
      ),
    ).toEqual({
      kind: "list",
      ports: [
        { port: 8123, primary: false },
        { port: 9000, primary: true },
        { port: 9009, primary: false },
      ],
    });
  });

  it("reports pending discovery rather than inventing a port", () => {
    expect(openPortsDisplay(privateService({ openPorts: [] }))).toEqual({
      kind: "pending",
    });
  });

  it("adds nothing for a single open port that is the primary", () => {
    expect(
      openPortsDisplay(privateService({ port: 8080, openPorts: tcp(8080) })),
    ).toEqual({ kind: "none" });
  });

  // w3/m273: a configured (or native) private service also serves the
  // listeners its instances bind.
  it("lists a configured private service's runtime listeners beside its port", () => {
    expect(
      openPortsDisplay(
        privateService({ port: 5432, openPorts: tcp(5432, 9187) }),
      ),
    ).toEqual({
      kind: "list",
      ports: [
        { port: 5432, primary: true },
        { port: 9187, primary: false },
      ],
    });
  });

  it("still lists a lone port that is not the primary", () => {
    expect(
      openPortsDisplay(privateService({ port: 8080, openPorts: tcp(9000) })),
    ).toEqual({ kind: "list", ports: [{ port: 9000, primary: false }] });
  });

  it.each([
    ["a private service whose ports were not read", privateService({})],
    [
      "a web service, even one serving several ports",
      { type: "web_service", port: 3000, openPorts: tcp(3000, 9000) },
    ],
    [
      "a background worker",
      { type: "background_worker", port: null, openPorts: null },
    ],
    [
      "a fixture that predates the field",
      { type: "private_service", port: 8080 },
    ],
  ] satisfies [string, PortFacts][])("adds nothing for %s", (_name, facts) => {
    expect(openPortsDisplay(facts)).toEqual({ kind: "none" });
  });
});

// w4/433: a saved configured image port is published only at promotion, so
// the Settings row reads configuredPort, falling back to port.
describe("settingsPort", () => {
  it("shows the saved port while the published one awaits promotion", () => {
    expect(settingsPort({ port: 3001, configuredPort: 3000 })).toBe(3000);
  });

  it("falls back to port when configuredPort is absent", () => {
    expect(settingsPort({ port: 8080, configuredPort: null })).toBe(8080);
    expect(settingsPort({ port: 8080 })).toBe(8080);
  });
});
