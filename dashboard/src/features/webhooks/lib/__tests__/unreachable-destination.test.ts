import { describe, expect, it } from "vitest";
import {
  isBlockedDestinationError,
  isUnreachableWebhookDestination,
} from "../unreachable-destination";

// w21/046: the host vectors are the delivery guard's (lego/types/netutil
// TestUnsafeOriginIP and the static-site rewrite host check pin the same ones).
describe("isUnreachableWebhookDestination", () => {
  it.each([
    "https://localhost/x",
    "https://localhost:3000/hook",
    "https://LOCALHOST/x",
    "https://app.localhost/x",
    "https://127.0.0.1/x",
    "https://127.255.255.254/x",
    "https://10.1.2.3/x",
    "https://172.16.0.1/x",
    "https://172.31.255.255/x",
    "https://192.168.1.1/x",
    "https://169.254.169.254/latest/meta-data",
    "https://100.64.0.1/x",
    "https://0.0.0.0/x",
    "https://198.18.0.1/x",
    "https://224.0.0.1/x",
    "https://255.255.255.255/x",
    "https://[::1]/x",
    "https://[::]/x",
    "https://[fe80::1]/x",
    "https://[fd00::1]/x",
    "https://[ff02::1]/x",
    "https://[::ffff:127.0.0.1]/x",
    "https://[::ffff:10.0.0.1]/x",
    "https://[64:ff9b::7f00:1]/x",
    "https://x.svc.cluster.local/x",
    "https://db.internal/x",
    "https://printer.lan/x",
    "https://nas.local/x",
    "https://x.home.arpa/x",
    "https://svc/x",
    "https://intranet/x",
    "https://./x",
    // The URL parser reads these numeric spellings as 127.0.0.1.
    "https://2130706433/x",
    "https://0x7f.1/x",
  ])("flags %s", (url) => {
    expect(isUnreachableWebhookDestination(url)).toBe(true);
  });

  it.each([
    "https://example.com/hook",
    "https://example.com:22/x",
    "https://example.com./x",
    "https://hooks.example.co.uk/x",
    "https://8.8.8.8/x",
    "https://172.15.0.1/x",
    "https://172.32.0.1/x",
    "https://100.63.255.255/x",
    "https://11.0.0.1/x",
    "https://[2606:4700::1111]/x",
    "https://[::ffff:8.8.8.8]/x",
    "https://xn--bcher-kva.de/x",
    "https://bücher.de/x",
    // Not a URL, or no host to judge: the form's own HTTPS message speaks.
    "",
    "not a url",
    "http//example.com",
  ])("accepts %s", (url) => {
    expect(isUnreachableWebhookDestination(url)).toBe(false);
  });

  it("judges the trimmed value", () => {
    expect(isUnreachableWebhookDestination("  https://localhost/x  ")).toBe(
      true,
    );
  });
});

describe("isBlockedDestinationError", () => {
  it("matches the delivery guard's refusal inside the transport error", () => {
    expect(
      isBlockedDestinationError(
        'Post "https://localhost:3000/…": destination resolves to a blocked address',
      ),
    ).toBe(true);
    expect(isBlockedDestinationError("dial tcp: i/o timeout")).toBe(false);
    expect(isBlockedDestinationError("")).toBe(false);
  });
});
