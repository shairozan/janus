// Module augmentation: teach TypeScript that our Session and JWT carry the
// Cognito access token (set in the auth.ts callbacks).
import "next-auth";
import "next-auth/jwt";

declare module "next-auth" {
  interface Session {
    accessToken?: string;
    error?: "RefreshAccessTokenError";
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string;
    accessTokenExpires?: number; // epoch seconds
    refreshToken?: string;
    error?: "RefreshAccessTokenError";
  }
}
