import { Link } from "@tanstack/react-router";
import { Home, ArrowLeft, AlertTriangle } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ErrorComponentProps } from "@tanstack/react-router";
import {
  DashboardDocumentTitle,
  translatedTitle,
} from "@/common/lib/document-head";
import { reportRouteError } from "@/common/lib/report-route-error";

/**
 * Global error page component
 * Displays a user-friendly error page when route validation or other errors occur
 */
export default function ErrorPage({ error, reset }: ErrorComponentProps) {
  const { t } = useTranslations();

  // w4/m88: SSR document failures must reach the pod stream (k9s/Loki). The
  // isomorphic helper no-ops in the browser so client navigations stay quiet.
  void reportRouteError(error, 500);

  const handleGoBack = () => {
    window.history.back();
  };

  const errorMessage = error?.message || t("common.errorDefaultMessage");

  return (
    <>
      <DashboardDocumentTitle title={translatedTitle("common.errorTitle")} />
      <div className="min-h-screen flex items-center justify-center bg-background px-4 sm:px-6 lg:px-8">
        <div className="w-full max-w-2xl text-center space-y-8">
          {/* Icon Section */}
          <div className="flex justify-center">
            <div className="rounded-full bg-destructive/10 p-6">
              <AlertTriangle className="h-16 w-16 text-destructive" />
            </div>
          </div>

          {/* Content Section */}
          <div className="space-y-4">
            <h1 className="text-4xl font-bold tracking-tight text-foreground">
              {t("common.errorTitle")}
            </h1>
            <p className="text-lg text-muted-foreground max-w-md mx-auto">
              {errorMessage}
            </p>
          </div>

          {/* Action Buttons */}
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4 pt-4">
            <Button
              asChild
              variant="default"
              size="lg"
              className="min-w-[140px]"
            >
              <Link to="/">
                <Home className="mr-2 h-4 w-4" />
                {t("common.goHome")}
              </Link>
            </Button>
            <Button
              variant="outline"
              size="lg"
              onClick={handleGoBack}
              className="min-w-[140px]"
            >
              <ArrowLeft className="mr-2 h-4 w-4" />
              {t("common.goBack")}
            </Button>
            <Button
              variant="outline"
              size="lg"
              onClick={reset}
              className="min-w-[140px]"
            >
              {t("common.tryAgain")}
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}
