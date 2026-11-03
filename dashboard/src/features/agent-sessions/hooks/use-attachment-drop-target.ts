import { useState } from "react";

/**
 * Drag-and-drop and clipboard-paste handlers for a composer surface. Files in
 * a paste (a screenshot) are attached; the paste reaches the editor unless it
 * carried files only, so mixed text-and-file clipboards keep their text.
 */
export function useAttachmentDropTarget(
  onFiles: (files: File[]) => void,
  enabled: boolean,
) {
  const [dragging, setDragging] = useState(false);
  const hasFiles = (e: React.DragEvent) =>
    Array.from(e.dataTransfer.types).includes("Files");
  return {
    dragging: enabled && dragging,
    handlers: enabled
      ? {
          onDragOver: (e: React.DragEvent) => {
            if (!hasFiles(e)) return;
            e.preventDefault();
            setDragging(true);
          },
          onDragLeave: (e: React.DragEvent) => {
            if (e.currentTarget.contains(e.relatedTarget as Node | null))
              return;
            setDragging(false);
          },
          onDrop: (e: React.DragEvent) => {
            if (!hasFiles(e)) return;
            e.preventDefault();
            setDragging(false);
            onFiles(Array.from(e.dataTransfer.files));
          },
          onPasteCapture: (e: React.ClipboardEvent) => {
            const files = Array.from(e.clipboardData?.files ?? []);
            if (files.length === 0) return;
            onFiles(files);
            const types = Array.from(e.clipboardData.types ?? []);
            if (!types.includes("text/plain")) e.preventDefault();
          },
        }
      : {},
  };
}
