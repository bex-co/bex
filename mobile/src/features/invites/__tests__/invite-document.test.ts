import fs from "fs";
import path from "path";
import { Kind, parse, print } from "graphql";

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
    const sources = [
      "invite-screen.tsx",
      "invite-provider.tsx",
      "invite-link-bootstrap.ts",
      "invite-controller.ts",
    ]
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
  });
});
