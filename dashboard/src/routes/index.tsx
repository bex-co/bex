import { createFileRoute, Link } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
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
// `beancount-cms` is the one real, running App among these samples (its
// `.onbex.co` host is `beancount-cms-v2` — the two are not the same string);
// `hasLiveMetrics` gates the Metrics PoC link (w3/m3) to it alone — the other
// rows are fake ids bex-api has never heard of, and linking them too would
// route to a live "app not found" GraphQL error the metrics page doesn't yet
// render distinctly from "no data".
const sampleServices = [
  {
    id: "beancount-cms",
    name: "beancount-cms",
    phase: "running",
    url: "https://beancount-cms-v2.onbex.co",
    hasLiveMetrics: true,
  },
  {
    id: "eden-cms-v2",
    name: "eden-cms-v2",
    phase: "running",
    url: "https://eden-cms-v2.onbex.co",
    hasLiveMetrics: false,
  },
  {
    id: "hello-go",
    name: "hello-go",
    phase: "running",
    url: "https://hello-go.onbex.co",
    hasLiveMetrics: false,
  },
  {
    id: "worker-queue",
    name: "worker-queue",
    phase: "suspended",
    url: null,
    hasLiveMetrics: false,
  },
];

function phaseVariant(phase: string): "default" | "secondary" | "outline" {
  if (phase === "running") return "default";
  if (phase === "suspended") return "secondary";
  return "outline";
}

const serviceStats = [
  { labelKey: "services.statTotal", value: sampleServices.length },
  {
    labelKey: "services.statRunning",
    value: sampleServices.filter((s) => s.phase === "running").length,
  },
  {
    labelKey: "services.statSuspended",
    value: sampleServices.filter((s) => s.phase === "suspended").length,
  },
] as const;

const PHASE_LABEL_KEYS: Record<string, keyof typeof en> = {
  running: "services.statusRunning",
  suspended: "services.statusSuspended",
};

export function HomePage() {
  const { t } = useTranslations();

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {serviceStats.map((stat) => (
              <Card key={stat.labelKey}>
                <CardHeader>
                  <CardDescription>{t(stat.labelKey)}</CardDescription>
                  <CardTitle className="text-2xl tabular-nums">
                    {stat.value}
                  </CardTitle>
                </CardHeader>
              </Card>
            ))}
          </div>
          <Card>
            <CardHeader>
              <CardTitle>{t("services.cardTitle")}</CardTitle>
              <CardDescription>{t("services.sampleDataNotice")}</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("services.colName")}</TableHead>
                    <TableHead>{t("services.colStatus")}</TableHead>
                    <TableHead>{t("services.colUrl")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sampleServices.map((service) => (
                    <TableRow key={service.id}>
                      <TableCell className="font-medium">
                        {service.hasLiveMetrics ? (
                          <Link
                            to="/services/$serviceId/metrics"
                            params={{ serviceId: service.id }}
                            className="hover:underline"
                          >
                            {service.name}
                          </Link>
                        ) : (
                          service.name
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={phaseVariant(service.phase)}>
                          {t(PHASE_LABEL_KEYS[service.phase])}
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
