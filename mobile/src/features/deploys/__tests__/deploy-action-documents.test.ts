import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("mobile deploy action GraphQL documents", () => {
  it("exposes only pre-parameterized mobile mutations", () => {
    const source = readFileSync(
      join(__dirname, "../api/deploy-actions.graphql"),
      "utf8",
    );
    expect(source.includes("MobileTriggerDeploy($serviceId: String!)")).toBe(
      true,
    );
    expect(
      source.includes(
        "MobileCancelDeploy($serviceId: String!, $deployId: String!)",
      ),
    ).toBe(true);
    expect(
      source.includes(
        "MobileRollbackService($serviceId: String!, $deployId: String!)",
      ),
    ).toBe(true);
    for (const forbidden of [
      "commitId",
      "deployMode",
      "imageUrl",
      "command",
      "clearCache",
      "delete",
    ]) {
      expect(source.includes(forbidden)).toBe(false);
    }
  });
});
