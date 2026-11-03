import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { useAttachmentDropTarget } from "@/features/agent-sessions/hooks/use-attachment-drop-target";

function Target({
  onFiles,
  enabled = true,
}: {
  onFiles: (files: File[]) => void;
  enabled?: boolean;
}) {
  const { handlers } = useAttachmentDropTarget(onFiles, enabled);
  return <div data-testid="target" {...handlers} />;
}

function paste(types: string[], files: File[]) {
  const event = new Event("paste", { bubbles: true, cancelable: true });
  Object.assign(event, { clipboardData: { files, types } });
  screen.getByTestId("target").dispatchEvent(event);
  return event;
}

const image = new File(["png"], "image.png", { type: "image/png" });

describe("useAttachmentDropTarget paste", () => {
  it("consumes a files-only paste (a screenshot)", () => {
    const onFiles = vi.fn();
    render(<Target onFiles={onFiles} />);
    const event = paste(["Files"], [image]);
    expect(onFiles).toHaveBeenCalledWith([image]);
    expect(event.defaultPrevented).toBe(true);
  });

  it("attaches the file but lets the text of a mixed paste through", () => {
    const onFiles = vi.fn();
    render(<Target onFiles={onFiles} />);
    const event = paste(["Files", "text/plain"], [image]);
    expect(onFiles).toHaveBeenCalledWith([image]);
    expect(event.defaultPrevented).toBe(false);
  });

  it("leaves a text-only paste alone", () => {
    const onFiles = vi.fn();
    render(<Target onFiles={onFiles} />);
    const event = paste(["text/plain"], []);
    expect(onFiles).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });

  it("does nothing while disabled (a send is in flight)", () => {
    const onFiles = vi.fn();
    render(<Target onFiles={onFiles} enabled={false} />);
    const event = paste(["Files"], [image]);
    expect(onFiles).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });
});
