import { Link } from "@tanstack/react-router";
import { Home, ArrowLeft, AlertTriangle } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import type { ErrorComponentProps } from "@tanstack/react-router";

/**
 * Global error page component
 * Displays a user-friendly error page when route validation or other errors occur
 */
export default function ErrorPage({ error, reset }: ErrorComponentProps) {
  const handleGoBack = () => {
    window.history.back();
  };

  const errorMessage = error?.message || "Something went wrong.";

  return (
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
            Something went wrong
          </h1>
          <p className="text-lg text-muted-foreground max-w-md mx-auto">
            {errorMessage}
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-4 pt-4">
          <Button asChild variant="default" size="lg" className="min-w-[140px]">
            <Link to="/">
              <Home className="mr-2 h-4 w-4" />
              Go home
            </Link>
          </Button>
          <Button
            variant="outline"
            size="lg"
            onClick={handleGoBack}
            className="min-w-[140px]"
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Go back
          </Button>
          <Button
            variant="outline"
            size="lg"
            onClick={reset}
            className="min-w-[140px]"
          >
            Try again
          </Button>
        </div>
      </div>
    </div>
  );
}
