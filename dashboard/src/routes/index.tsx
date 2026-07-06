import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Badge } from "@/common/components/ui/badge.tsx";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table.tsx";

export const Route = createFileRoute("/")({
  component: HomePage,
  beforeLoad: requireAuth("/"),
  head: () => ({
    meta: [{ title: "bex dashboard" }],
  }),
});

// Sample data shaped like bex-api's `GET /v1/services` response
// (docs/bex-api.md) — replace with a real Apollo query once wired up.
const sampleServices = [
  {
    id: "eden-cms-v2",
    name: "eden-cms-v2",
    phase: "running",
    url: "https://eden-cms-v2.onbex.co",
  },
  {
    id: "hello-go",
    name: "hello-go",
    phase: "running",
    url: "https://hello-go.onbex.co",
  },
  { id: "worker-queue", name: "worker-queue", phase: "suspended", url: null },
];

function phaseVariant(phase: string): "default" | "secondary" | "outline" {
  if (phase === "running") return "default";
  if (phase === "suspended") return "secondary";
  return "outline";
}

const serviceStats = [
  { label: "Total services", value: sampleServices.length },
  {
    label: "Running",
    value: sampleServices.filter((s) => s.phase === "running").length,
  },
  {
    label: "Suspended",
    value: sampleServices.filter((s) => s.phase === "suspended").length,
  },
];

export function HomePage() {
  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {serviceStats.map((stat) => (
              <Card key={stat.label}>
                <CardHeader>
                  <CardDescription>{stat.label}</CardDescription>
                  <CardTitle className="text-2xl tabular-nums">
                    {stat.value}
                  </CardTitle>
                </CardHeader>
              </Card>
            ))}
          </div>
          <Card>
            <CardHeader>
              <CardTitle>Services</CardTitle>
              <CardDescription>
                Sample data — this scaffold isn't wired to bex-api yet.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>URL</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sampleServices.map((service) => (
                    <TableRow key={service.id}>
                      <TableCell className="font-medium">
                        {service.name}
                      </TableCell>
                      <TableCell>
                        <Badge variant={phaseVariant(service.phase)}>
                          {service.phase}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {service.url ?? "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  );
}
