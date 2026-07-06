import * as React from "react";
import * as TabsPrimitive from "@radix-ui/react-tabs";

import { cn } from "@/common/lib/utils/utils.ts";

type TabsVariant = "default" | "underline";

const TabsVariantContext = React.createContext<TabsVariant>("default");

function Tabs({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Root>) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      className={cn("flex flex-col gap-2", className)}
      {...props}
    />
  );
}

function TabsList({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List> & {
  variant?: TabsVariant;
}) {
  return (
    <TabsVariantContext.Provider value={variant}>
      <TabsPrimitive.List
        data-slot="tabs-list"
        className={cn(
          "inline-flex items-center justify-center",
          variant === "default" &&
            "bg-muted text-muted-foreground h-9 w-fit rounded-lg p-[3px]",
          variant === "underline" && "h-auto w-full border-b border-border",
          className,
        )}
        {...props}
      />
    </TabsVariantContext.Provider>
  );
}

function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  const variant = React.useContext(TabsVariantContext);

  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      className={cn(
        "inline-flex items-center justify-center gap-1.5 whitespace-nowrap font-medium transition-all",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:outline-ring",
        "disabled:pointer-events-none disabled:opacity-50",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        "cursor-pointer",
        // Default variant styles (pill-style)
        variant === "default" &&
          cn(
            "h-[calc(100%-1px)] flex-1 rounded-md border border-transparent px-2 py-1 text-sm",
            "text-foreground dark:text-muted-foreground",
            "data-[state=active]:bg-background data-[state=active]:shadow-sm",
            "dark:data-[state=active]:text-foreground dark:data-[state=active]:border-input dark:data-[state=active]:bg-input/30",
            "transition-[color,box-shadow] focus-visible:ring-[3px] focus-visible:outline-1",
          ),
        // Underline variant styles (GitHub-style)
        variant === "underline" &&
          cn(
            "border-b-2 border-transparent px-4 py-3 text-base -mb-px",
            "text-muted-foreground",
            "hover:border-muted-foreground/50 hover:text-foreground",
            "data-[state=active]:border-primary data-[state=active]:text-foreground",
          ),
        className,
      )}
      {...props}
    />
  );
}

function TabsContent({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      data-slot="tabs-content"
      className={cn("flex-1 outline-none", className)}
      {...props}
    />
  );
}

export { Tabs, TabsList, TabsTrigger, TabsContent };
