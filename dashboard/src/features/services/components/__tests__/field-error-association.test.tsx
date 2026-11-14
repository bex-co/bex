import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CreateEnvVarEditor } from "../create-env-var-editor";
import { CreateSecretFileEditor } from "../create-secret-file-editor";

// w4/461: an invalid field's visible message is announced (role="alert") and
// tied to its input with aria-describedby, per row.
describe("create-time editor field errors", () => {
  it("env var key error is described and announced", () => {
    render(
      <CreateEnvVarEditor
        rows={[
          { key: "OK_KEY", value: "" },
          { key: "bad key", value: "" },
        ]}
        onChange={() => undefined}
      />,
    );
    const inputs = screen.getAllByRole("textbox", { name: /key/i });
    const invalid = inputs.find(
      (i) => i.getAttribute("aria-invalid") === "true",
    );
    expect(invalid).toBeDefined();
    const alert = screen.getByRole("alert");
    expect(invalid).toHaveAttribute("aria-describedby", alert.id);
    // The valid row carries no description.
    const valid = inputs.find(
      (i) => i !== invalid && i.getAttribute("aria-label")?.match(/key/i),
    );
    expect(valid).not.toHaveAttribute("aria-describedby");
  });

  it("secret file name error is described and announced", () => {
    render(
      <CreateSecretFileEditor
        rows={[{ name: "bad/name", content: "" }]}
        onChange={() => undefined}
      />,
    );
    const input = screen
      .getAllByRole("textbox")
      .find((i) => i.getAttribute("aria-invalid") === "true");
    const alert = screen.getByRole("alert");
    expect(input).toHaveAttribute("aria-describedby", alert.id);
  });

  // w4/658: files past the 1 MiB total are marked before Submit, with the
  // Environment editor's sentence.
  it("secret files past the 1 MiB total are described and announced", () => {
    const half = "x".repeat(512 * 1024);
    render(
      <CreateSecretFileEditor
        rows={[
          { name: "a.pem", content: half },
          { name: "b.pem", content: half + "x" },
        ]}
        onChange={() => undefined}
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/combined size to 1 MiB/);
    const contents = screen.getAllByRole("textbox", { name: /content/i });
    expect(contents).toHaveLength(2);
    for (const content of contents) {
      expect(content).toHaveAttribute("aria-invalid", "true");
      expect(content).toHaveAttribute("aria-describedby", alert.id);
    }
  });

  it("secret files within the 1 MiB total carry no limit alert", () => {
    const half = "x".repeat(512 * 1024);
    render(
      <CreateSecretFileEditor
        rows={[
          { name: "a.pem", content: half },
          { name: "b.pem", content: half },
        ]}
        onChange={() => undefined}
      />,
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
