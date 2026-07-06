import type { ReactNode } from "react";
import { cn } from "@/common/lib/utils/utils";

interface PageHeaderProps {
  title: ReactNode;
  description: ReactNode;
  className?: string;
}

export function PageHeader({ title, description, className }: PageHeaderProps) {
  return (
    <div className={cn("hidden md:flex md:flex-col space-y-1", className)}>
      <h1 className="text-lg sm:text-xl font-normal text-muted-foreground">
        {title}
      </h1>
      <p className="text-sm text-muted-foreground leading-relaxed">
        {description}
      </p>
    </div>
  );
}
