import { useState } from "react";
import {
  Link,
  Outlet,
  createFileRoute,
  useRouter,
  useRouterState,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { BlueprintStatusBadge } from "@/features/blueprints/components/blueprint-status-badge";
import { useBlueprints } from "@/features/blueprints/hooks/use-blueprints";
import { useCreateBlueprint } from "@/features/blueprints/hooks/use-create-blueprint";
import { formatRelativeAge } from "@/features/services/lib/format";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { protectedServiceName } from "@/features/services/lib/protected-confirmation";

export const Route = createFileRoute("/blueprints")({
  component: BlueprintsPage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("blueprints.pageTitle", match),
});

export function BlueprintsPage() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });

  if (pathname !== "/blueprints") return <Outlet />;

  return <BlueprintsListPage />;
}

function BlueprintsListPage() {
  const { t } = useTranslations();
  const router = useRouter();
  const { blueprints, loading, error } = useBlueprints();
  const { create, busy } = useCreateBlueprint();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [repo, setRepo] = useState("");
  const [branch, setBranch] = useState("main");
  const [path, setPath] = useState("bex.yml");
  const [name, setName] = useState("");
  const [protectedConfirmation, setProtectedConfirmation] = useState<
    string | null
  >(null);

  const showSkeleton = loading && blueprints.length === 0;

  function openDialog() {
    setRepo("");
    setBranch("main");
    setPath("bex.yml");
    setName("");
    setDialogOpen(true);
  }

  async function handleCreate(confirmation?: string) {
    const result = await create(repo, branch, path, name, confirmation);
    if (result.status === "confirmation_required") {
      setProtectedConfirmation(result.confirmation);
      return;
    }
    if (result.status === "success") {
      setDialogOpen(false);
      setProtectedConfirmation(null);
      void router.navigate({
        to: "/blueprints/$blueprintId",
        params: { blueprintId: result.blueprint.id },
      });
    }
  }

  return (
    <DashboardLayout>
      <div className="flex items-center justify-between border-b px-4 py-4 sm:px-6">
        <h1 className="text-xl font-semibold">{t("blueprints.pageTitle")}</h1>
        <Button size="sm" onClick={openDialog}>
          {t("blueprints.createButton")}
        </Button>
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {error && blueprints.length === 0 ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("blueprints.errorTitle")}</p>
            </div>
          ) : showSkeleton ? (
            <Skeleton className="h-40 w-full" />
          ) : blueprints.length === 0 ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("blueprints.emptyTitle")}</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {t("blueprints.emptyBody")}
              </p>
            </div>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>{t("blueprints.cardTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("blueprints.colName")}</TableHead>
                      <TableHead>{t("blueprints.colRepo")}</TableHead>
                      <TableHead>{t("blueprints.colBranch")}</TableHead>
                      <TableHead>{t("blueprints.colStatus")}</TableHead>
                      <TableHead>{t("blueprints.colUpdated")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {blueprints.map((bp) => (
                      <TableRow key={bp.id}>
                        <TableCell className="font-medium">
                          <Link
                            to="/blueprints/$blueprintId"
                            params={{ blueprintId: bp.id }}
                            className="hover:underline"
                          >
                            {bp.name}
                          </Link>
                        </TableCell>
                        <TableCell className="max-w-[200px] truncate text-sm text-muted-foreground">
                          {bp.repo}
                        </TableCell>
                        <TableCell className="text-sm">{bp.branch}</TableCell>
                        <TableCell>
                          <BlueprintStatusBadge status={bp.status} />
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {bp.updatedAt ? formatRelativeAge(bp.updatedAt) : "—"}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </div>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("blueprints.createTitle")}</DialogTitle>
            <DialogDescription>
              {t("blueprints.emptyBody")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="bp-repo">
                {t("blueprints.createRepoLabel")}
              </Label>
              <Input
                id="bp-repo"
                value={repo}
                onChange={(e) => setRepo(e.target.value)}
                placeholder={t("blueprints.createRepoPlaceholder")}
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="bp-branch">
                {t("blueprints.createBranchLabel")}
              </Label>
              <Input
                id="bp-branch"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                placeholder={t("blueprints.createBranchPlaceholder")}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="bp-path">
                {t("blueprints.createPathLabel")}
              </Label>
              <Input
                id="bp-path"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder={t("blueprints.createPathPlaceholder")}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="bp-name">
                {t("blueprints.createNameLabel")}
              </Label>
              <Input
                id="bp-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my-stack"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDialogOpen(false)}
              disabled={busy}
            >
              {t("blueprints.createCancel")}
            </Button>
            <Button
              onClick={() => void handleCreate()}
              disabled={busy || !repo.trim()}
            >
              {t("blueprints.createAction")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ProtectedConfirmationDialog
        key={protectedConfirmation ? `open:${protectedConfirmation}` : "closed"}
        open={protectedConfirmation !== null}
        resourceName={
          protectedConfirmation
            ? protectedServiceName(protectedConfirmation)
            : (name || repo)
        }
        requiredConfirmation={protectedConfirmation ?? ""}
        actionLabel={t("blueprints.createAction")}
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirmation(null)}
        onConfirm={async (confirmation) => {
          await handleCreate(confirmation);
        }}
      />
    </DashboardLayout>
  );
}
