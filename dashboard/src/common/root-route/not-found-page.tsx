import { Link } from "@tanstack/react-router";
import { Home, ArrowLeft } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";

/**
 * 404 Not Found page component
 * Displays a modern and clean error page when a route is not found
 */
export default function NotFoundPage() {
  const handleGoBack = () => {
    window.history.back();
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4 sm:px-6 lg:px-8">
      <div className="w-full max-w-2xl text-center space-y-8">
        {/* Content Section */}
        <div className="space-y-4">
          <h1 className="text-8xl font-bold tracking-tight text-foreground">
            404
          </h1>
          <h2 className="text-3xl font-semibold text-foreground">
            Page not found
          </h2>
          <p className="text-lg text-muted-foreground max-w-md mx-auto">
            The page you're looking for doesn't exist or has been moved.
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
        </div>
      </div>
    </div>
  );
}
