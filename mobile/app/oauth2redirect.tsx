import { Redirect } from "expo-router";

// A cold-start callback cannot recover the in-memory PKCE verifier. The active
// AuthSession consumes valid callbacks before Expo Router sees this route.
export default function OrphanedOAuthRedirect() {
  return <Redirect href="/sign-in" />;
}
