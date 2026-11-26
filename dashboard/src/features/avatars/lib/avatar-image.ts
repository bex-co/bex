// Making the image an avatar is (w1/m648, docs/render-artifacts/avatars.md):
// Render uploads a 128x128 JPEG whatever the source, a cropped photo drawn on
// white or a monogram drawn on its color.

import {
  monogramColor,
  type Monogram,
} from "@/features/avatars/lib/avatar-utils";

/** The side of every avatar image the dashboard uploads. */
const AVATAR_IMAGE_PIXELS = 128;

/** Render refuses a file whose data URL is longer than this (20 MiB). */
export const MAX_AVATAR_DATA_URL_LENGTH = 20_971_520;

/** The two file types the picker accepts, as Render's input lists them. */
export const AVATAR_FILE_TYPES = ["image/png", "image/jpeg"] as const;

/**
 * Where the crop dialog's circle sits over the image, in the image's own
 * pixels. The dialog fits the image inside its square viewport (contain), and
 * the circle's diameter is the fitted image's shorter side, as Render's
 * cropper does; zoom scales the image about the viewport's centre and the
 * offset is how far it was dragged, both in viewport pixels.
 */
export interface CropView {
  imageWidth: number;
  imageHeight: number;
  viewport: number;
  zoom: number;
  offsetX: number;
  offsetY: number;
}

type Fit = Pick<CropView, "imageWidth" | "imageHeight" | "viewport">;

/** The scale that fits the image inside the viewport (contain). */
export function fitScale(view: Fit): number {
  return view.viewport / Math.max(view.imageWidth, view.imageHeight);
}

/** The circle's diameter in viewport pixels. */
export function cropDiameter(view: Fit): number {
  return Math.min(view.imageWidth, view.imageHeight) * fitScale(view);
}

/** The square of the image under the crop circle. */
export function cropArea(view: CropView): {
  x: number;
  y: number;
  size: number;
} {
  const scale = fitScale(view) * view.zoom;
  const size = cropDiameter(view) / scale;
  const centreX = view.imageWidth / 2 - view.offsetX / scale;
  const centreY = view.imageHeight / 2 - view.offsetY / scale;
  return { x: centreX - size / 2, y: centreY - size / 2, size };
}

function newCanvas(): {
  canvas: HTMLCanvasElement;
  context: CanvasRenderingContext2D;
} {
  const canvas = document.createElement("canvas");
  canvas.width = AVATAR_IMAGE_PIXELS;
  canvas.height = AVATAR_IMAGE_PIXELS;
  const context = canvas.getContext("2d");
  if (!context) throw new Error("Could not get 2D context");
  return { canvas, context };
}

function toJpeg(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) resolve(blob);
      else reject(new Error("Cannot create blob from canvas"));
    }, "image/jpeg");
  });
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("Cannot load image"));
    image.src = src;
  });
}

/** The crop, drawn on white into a 128x128 JPEG. */
export async function cropToJpeg(src: string, view: CropView): Promise<Blob> {
  const image = await loadImage(src);
  const { canvas, context } = newCanvas();
  context.fillStyle = "white";
  context.fillRect(0, 0, canvas.width, canvas.height);
  const area = cropArea(view);
  context.drawImage(
    image,
    area.x,
    area.y,
    area.size,
    area.size,
    0,
    0,
    canvas.width,
    canvas.height,
  );
  return toJpeg(canvas);
}

/** A monogram drawn square into a 128x128 JPEG, as the dialog previews it. */
export function monogramToJpeg(monogram: Monogram): Promise<Blob> {
  const { canvas, context } = newCanvas();
  const color = monogramColor(monogram.colorIndex);
  context.fillStyle = color.background;
  context.fillRect(0, 0, canvas.width, canvas.height);
  const family =
    typeof document !== "undefined"
      ? getComputedStyle(document.body).fontFamily || "sans-serif"
      : "sans-serif";
  context.fillStyle = color.foreground;
  context.font = `500 ${Math.round(canvas.width * 0.44)}px ${family}`;
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.fillText(monogram.letter, canvas.width / 2, canvas.height / 2 + 2);
  return toJpeg(canvas);
}

/** A file or blob as a data URL; rejects when the browser cannot read it. */
export function readAsDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () =>
      typeof reader.result === "string"
        ? resolve(reader.result)
        : reject(new Error("Unable to read avatar image file as string"));
    reader.onerror = () => reject(reader.error ?? new Error("read failed"));
    reader.readAsDataURL(blob);
  });
}
