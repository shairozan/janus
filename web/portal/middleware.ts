// Route protection. The matcher scopes this middleware to the authenticated
// portal areas; any unauthenticated request to them is bounced to /login.
// Auth.js's `auth` wrapper exposes the session as req.auth (read from the
// session cookie — no network call).
import { auth } from "@/auth";

export default auth((req) => {
  if (!req.auth) {
    const url = new URL("/login", req.nextUrl.origin);

    return Response.redirect(url);
  }
});

export const config = {
  matcher: [
    "/me",
    "/me/:path*",
    "/orgs/:path*",
    "/admin/:path*",
    "/onboarding",
    "/onboarding/:path*",
  ],
};
