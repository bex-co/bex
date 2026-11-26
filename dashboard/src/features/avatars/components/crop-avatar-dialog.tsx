import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { Minus, Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Slider } from "@/common/components/ui/slider";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  cropDiameter,
  fitScale,
  type CropView,
} from "@/features/avatars/lib/avatar-image";

/** Render's zoom range and step. */
const MIN_ZOOM = 1;
const MAX_ZOOM = 5;
const ZOOM_STEP = 0.1;
/** How far one arrow key moves the image, in viewport pixels. */
const KEY_STEP = 8;
/** The viewport's side before it has been measured (Render's is 400). */
const DEFAULT_VIEWPORT = 400;

const clampZoom = (zoom: number) =>
  Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(zoom * 10) / 10));

/**
 * The cropper and its buttons. It mounts with each image, so a new image
 * starts centred at zoom 1 without an effect.
 */
function Cropper({
  imageDataURL,
  onCancel,
  onSave,
}: {
  imageDataURL: string;
  onCancel: () => void;
  onSave: (view: CropView) => Promise<void>;
}) {
  const { t } = useTranslations();
  const viewportRef = useRef<HTMLDivElement>(null);
  const drag = useRef<{ x: number; y: number } | null>(null);
  const [viewport, setViewport] = useState(DEFAULT_VIEWPORT);
  const [natural, setNatural] = useState<{ w: number; h: number } | null>(null);
  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const element = viewportRef.current;
    if (!element || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => {
      if (entry && entry.contentRect.width > 0) {
        setViewport(entry.contentRect.width);
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const fitted = natural && {
    imageWidth: natural.w,
    imageHeight: natural.h,
    viewport,
  };
  const fit = fitted ? fitScale(fitted) : 1;
  const diameter = fitted ? cropDiameter(fitted) : viewport;

  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    drag.current = { x: event.clientX, y: event.clientY };
    event.currentTarget.setPointerCapture?.(event.pointerId);
  };
  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const from = drag.current;
    if (!from) return;
    drag.current = { x: event.clientX, y: event.clientY };
    setOffset((o) => ({
      x: o.x + event.clientX - from.x,
      y: o.y + event.clientY - from.y,
    }));
  };
  const onPointerUp = () => {
    drag.current = null;
  };
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const moves: Record<string, [number, number]> = {
      ArrowLeft: [-KEY_STEP, 0],
      ArrowRight: [KEY_STEP, 0],
      ArrowUp: [0, -KEY_STEP],
      ArrowDown: [0, KEY_STEP],
    };
    const move = moves[event.key];
    if (move) {
      event.preventDefault();
      setOffset((o) => ({ x: o.x + move[0], y: o.y + move[1] }));
    } else if (event.key === "+" || event.key === "=") {
      event.preventDefault();
      setZoom((z) => clampZoom(z + ZOOM_STEP));
    } else if (event.key === "-") {
      event.preventDefault();
      setZoom((z) => clampZoom(z - ZOOM_STEP));
    }
  };

  const save = async () => {
    if (!natural) return;
    setSaving(true);
    try {
      await onSave({
        imageWidth: natural.w,
        imageHeight: natural.h,
        viewport,
        zoom,
        offsetX: offset.x,
        offsetY: offset.y,
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <div className="flex flex-col items-center gap-3">
        {/* eslint-disable-next-line jsx-a11y-x/no-noninteractive-element-interactions -- the crop area is the control: dragged to move the image, and focusable so the arrow keys move it too */}
        <div
          ref={viewportRef}
          role="group"
          // eslint-disable-next-line jsx-a11y-x/no-noninteractive-tabindex -- see above: keyboard users move the image from here
          tabIndex={0}
          aria-label={t("avatars.cropHint")}
          data-testid="avatar-cropper"
          className="relative aspect-square w-full max-w-[400px] cursor-move touch-none overflow-hidden rounded-md bg-muted outline-none focus-visible:ring-2 focus-visible:ring-ring"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          onKeyDown={onKeyDown}
          onWheel={(event) =>
            setZoom((z) =>
              clampZoom(z + (event.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP)),
            )
          }
        >
          <img
            src={imageDataURL}
            alt=""
            draggable={false}
            onLoad={(event) =>
              setNatural({
                w: event.currentTarget.naturalWidth,
                h: event.currentTarget.naturalHeight,
              })
            }
            className="pointer-events-none absolute top-1/2 left-1/2 max-w-none select-none"
            style={
              natural
                ? {
                    width: natural.w * fit,
                    height: natural.h * fit,
                    transform: `translate(-50%, -50%) translate(${offset.x}px, ${offset.y}px) scale(${zoom})`,
                  }
                : { visibility: "hidden" }
            }
          />
          {/* The round crop: everything outside it is dimmed. */}
          <div
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_0_0_9999px_rgba(0,0,0,0.5)]"
            style={{ width: diameter, height: diameter }}
          />
        </div>
        <div className="flex w-full max-w-[400px] items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t("avatars.zoomOut")}
            disabled={zoom <= MIN_ZOOM}
            onClick={() => setZoom((z) => clampZoom(z - ZOOM_STEP))}
          >
            <Minus className="size-4" />
          </Button>
          <Slider
            min={MIN_ZOOM}
            max={MAX_ZOOM}
            step={ZOOM_STEP}
            value={[zoom]}
            onValueChange={([value]) => setZoom(clampZoom(value ?? zoom))}
            thumbLabels={[t("avatars.cropHint")]}
            className="flex-1"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t("avatars.zoomIn")}
            disabled={zoom >= MAX_ZOOM}
            onClick={() => setZoom((z) => clampZoom(z + ZOOM_STEP))}
          >
            <Plus className="size-4" />
          </Button>
        </div>
      </div>
      <DialogFooter className="sm:justify-between">
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          disabled={saving}
        >
          {t("avatars.cancel")}
        </Button>
        <Button
          type="button"
          onClick={() => void save()}
          disabled={!natural || saving}
          data-testid="crop-save"
        >
          {t("avatars.save")}
        </Button>
      </DialogFooter>
    </>
  );
}

/**
 * Render's "Crop Avatar" dialog: the chosen image fitted in a square, a round
 * crop of aspect 1 over it, dragged to move (or moved with the arrow keys) and
 * zoomed from 1 to 5 by the slider, the buttons or the wheel. Save hands the
 * view to `onSave`, which draws the 128x128 JPEG.
 */
export function CropAvatarDialog({
  imageDataURL,
  onCancel,
  onSave,
}: {
  /** The image to crop; the dialog is open while there is one. */
  imageDataURL: string | null;
  onCancel: () => void;
  onSave: (view: CropView) => Promise<void>;
}) {
  const { t } = useTranslations();
  return (
    <Dialog
      open={imageDataURL !== null}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
    >
      <DialogContent className="sm:max-w-[464px]">
        <DialogHeader>
          <DialogTitle>{t("avatars.cropTitle")}</DialogTitle>
        </DialogHeader>
        {imageDataURL ? (
          <Cropper
            key={imageDataURL}
            imageDataURL={imageDataURL}
            onCancel={onCancel}
            onSave={onSave}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
