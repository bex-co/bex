import fs from "fs";
import path from "path";
import { Kind, parse, print } from "graphql";
import {
  VERIFIED_INVITE_ORIGIN,
  VERIFIED_INVITE_PATH,
} from "../invite-link-bootstrap";

describe("mobile invite operation", () => {
  it("contains only the direct acceptance mutation", () => {
    const source = fs.readFileSync(
      path.resolve(process.cwd(), "src/features/invites/api/invites.graphql"),
      "utf8",
    );
    const parsed = parse(source);
    const operations = parsed.definitions.filter(
      (definition) => definition.kind === Kind.OPERATION_DEFINITION,
    );
    expect(operations.length).toBe(1);
    expect(
      String(
        operations[0]?.kind === Kind.OPERATION_DEFINITION
          ? operations[0].operation
          : undefined,
      ),
    ).toBe("mutation");
    const document = print(parsed);
    expect(document.includes("acceptWorkspaceInvite")).toBe(true);
    expect(document.includes("workspaceId")).toBe(true);
    expect(
      /inviteWorkspaceMember|workspaceInvites|workspaceMembers|billing|plan|delete|remove/i.test(
        document,
      ),
    ).toBe(false);
  });

  it("keeps the bearer out of user-visible and non-secure utility paths", () => {
    const featureRoot = path.resolve(process.cwd(), "src/features/invites");
    const productionFiles = fs
      .readdirSync(featureRoot, { recursive: true })
      .map(String)
      .filter((file) => /\.(?:ts|tsx)$/.test(file))
      .filter((file) => !file.includes("__tests__"));
    const sources = productionFiles
      .map((file) => fs.readFileSync(path.join(featureRoot, file), "utf8"))
      .join("\n");
    expect(/console\.|AsyncStorage|Clipboard|\bShare\b/.test(sources)).toBe(
      false,
    );
    const screen = fs.readFileSync(
      path.join(featureRoot, "invite-screen.tsx"),
      "utf8",
    );
    expect(/pending\.token|state\.token|content\.token/.test(screen)).toBe(
      false,
    );
    const client = fs.readFileSync(
      path.join(featureRoot, "graphql-client.ts"),
      "utf8",
    );
    expect(client.includes('fetchPolicy: "no-cache"')).toBe(true);
  });

  it("keeps runtime source verification aligned with the OS claims", () => {
    const config = JSON.parse(
      fs.readFileSync(path.resolve(process.cwd(), "app.json"), "utf8"),
    ) as {
      expo: {
        ios: { associatedDomains: string[] };
        android: { intentFilters: unknown[] };
      };
    };
    const origin = new URL(VERIFIED_INVITE_ORIGIN);
    expect(config.expo.ios.associatedDomains).toEqual([
      `applinks:${origin.host}`,
    ]);
    expect(config.expo.android.intentFilters).toEqual([
      {
        action: "VIEW",
        autoVerify: true,
        data: [
          {
            scheme: origin.protocol.slice(0, -1),
            host: origin.host,
            path: VERIFIED_INVITE_PATH,
          },
        ],
        category: ["BROWSABLE", "DEFAULT"],
      },
    ]);

    const screen = fs.readFileSync(
      path.resolve(process.cwd(), "src/features/invites/invite-screen.tsx"),
      "utf8",
    );
    expect(screen.includes("useLinkingURL()")).toBe(true);
    expect(screen.includes("verifiedInviteToken(linkingURL, value)")).toBe(
      true,
    );
  });
});
