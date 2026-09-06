import * as React from "react";
import * as SliderPrimitive from "@radix-ui/react-slider";
import { cn } from "@/common/lib/utils/utils";

function Slider({
  className,
  thumbLabels,
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root> & {
  thumbLabels?: string[];
}) {
  return (
    <SliderPrimitive.Root
      className={cn(
        "relative flex w-full cursor-pointer touch-none items-center select-none data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50",
        className,
      )}
      {...props}
    >
      <SliderPrimitive.Track className="relative h-1.5 w-full grow overflow-hidden rounded-full bg-muted">
        <SliderPrimitive.Range className="absolute h-full bg-primary" />
      </SliderPrimitive.Track>
      {(props.value ?? props.defaultValue ?? [0]).map(
        (_: number, i: number) => (
          <SliderPrimitive.Thumb
            key={i}
            {...(thumbLabels?.[i] ? { "aria-label": thumbLabels[i] } : {})}
            className="block h-4 w-4 rounded-full border border-primary/50 bg-background shadow transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
          />
        ),
      )}
    </SliderPrimitive.Root>
  );
}

export { Slider };
