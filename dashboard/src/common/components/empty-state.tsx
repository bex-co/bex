import type { LucideIcon } from "lucide-react";
import {
  AlertCircle,
  Database,
  DatabaseZap,
  FileSpreadsheet,
  LockKeyhole,
  ScrollText,
} from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";

// A static map of the icon names `EmptyState` is actually passed app-wide,
// instead of `import * as Icons from "lucide-react"` + dynamic index — the
// namespace import + runtime lookup defeats tree-shaking and hoists the full
// ~1,900-icon barrel into the always-mounted entry chunk (w9/m60 t001).
// Adding a new `iconName` value means adding its named import here.
const ICONS: Record<string, LucideIcon> = {
  AlertCircle,
  Database,
  DatabaseZap,
  LockKeyhole,
  ScrollText,
};

interface EmptyStateProps {
  title: string;
  description: string;
  iconName?: string;
}

export function EmptyState({ title, description, iconName }: EmptyStateProps) {
  const Icon: LucideIcon =
    (iconName ? ICONS[iconName] : undefined) ?? FileSpreadsheet;

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
