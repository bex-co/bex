import { describe, expect, it } from "vitest";
import { GA_READY_EVENT } from "../../src/common/lib/google-analytics";
import {
  READY_EVENT,
  containerProblems,
  containerResource,
} from "../check-ga-container.mjs";

// The script cannot import the app's TypeScript; the two must name one event.
it("checks for the event the dashboard pushes", () => {
  expect(READY_EVENT).toBe(GA_READY_EVENT);
});

/** A container in the shape gtag.js embeds it, with the settings under test. */
function container({
  trigger = "bex_ga_ready",
  sendPageView = "false",
  history = false,
  userData = false,
} = {}) {
  return {
    version: "5",
    tags: [
      {
        function: "__ogt_auto_events",
        vtp_enableScroll: false,
        vtp_enableHistoryEvents: history,
        vtp_enableInPageEventsHidden: false,
      },
      { function: "__ogt_1p_data_v2", vtp_isEnabled: userData },
      {
        function: "__dest_ga",
        vtp_destinationId: "G-7X5XTVZ55N",
        vtp_configSettingsTable: [
          "list",
          [
            "map",
            "parameter",
            "send_page_view",
            "parameterValue",
            sendPageView,
          ],
        ],
      },
      { function: "__gct" },
    ],
    predicates: [
      { function: "_eq", arg0: ["macro", 0], arg1: trigger },
      { function: "_eq", arg0: ["macro", 0], arg1: "gtm.js" },
      { function: "_eq", arg0: ["macro", 0], arg1: "gtm.init" },
    ],
    rules: [
      [
        ["if", 0],
        ["add", 2],
      ],
      [
        ["if", 1],
        ["add", 3],
      ],
      [
        ["if", 2],
        ["add", 0, 1],
      ],
    ],
  };
}

describe("containerResource", () => {
  it("reads the resource object out of gtag.js", () => {
    const js = `var data = {\n"resource": ${JSON.stringify(container())}\n\n,"runtime":[[50,"__c"]]};`;
    expect(containerResource(js)).toEqual(container());
  });

  it("is null when gtag.js no longer has that shape", () => {
    expect(containerResource("window.gtag=function(){}")).toBeNull();
    expect(containerResource('"resource": {oops ,"runtime":[]')).toBeNull();
  });
});

describe("containerProblems", () => {
  it("passes the configured container", () => {
    expect(containerProblems(container())).toEqual([]);
  });

  it("fails a tag that fires at initialization", () => {
    expect(containerProblems(container({ trigger: "gtm.init" }))).toEqual([
      "the Google Analytics tag fires on [gtm.init], not only on bex_ga_ready",
    ]);
  });

  it("fails an automatic page view, history detection and user data", () => {
    expect(
      containerProblems(
        container({ sendPageView: "true", history: true, userData: true }),
      ),
    ).toEqual([
      "the Google Analytics tag does not set send_page_view=false",
      "automatic event detection is on: HistoryEvents",
      "user-provided data detection is on",
    ]);
  });

  it("fails a container without the Google Analytics tag", () => {
    expect(containerProblems({ tags: [], predicates: [], rules: [] })).toEqual([
      "the container has no Google Analytics tag",
    ]);
  });
});
