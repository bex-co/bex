import * as Icons from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { FileSpreadsheet } from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";

interface EmptyStateProps {
  title: string;
  description: string;
  iconName?: string;
}

export function EmptyState({ title, description, iconName }: EmptyStateProps) {
  const Icon: LucideIcon =
    iconName && iconName in Icons
      ? (Icons[iconName as keyof typeof Icons] as LucideIcon)
      : FileSpreadsheet;

  return (
    <Card>
      <CardContent className="flex items-center justify-center py-16">
        <div className="text-center">
          <Icon className="h-8 w-8 text-muted-foreground/50 mx-auto mb-3" />
          <p className="font-medium mb-1">{title}</p>
          <p className="text-muted-foreground text-sm">{description}</p>
        </div>
      </CardContent>
    </Card>
  );
}
