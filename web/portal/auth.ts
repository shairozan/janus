import NextAuth from "next-auth";
import type { JWT } from "next-auth/jwt";
import Cognito from "next-auth/providers/cognito";

// Cognito's token endpoint (the hosted-UI domain, not the issuer) — discovered
// once and cached. Used for refresh-token rotation.
let cachedTokenEndpoint: string | undefined;
async function cognitoTokenEndpoint(): Promise<string> {
  if (cachedTokenEndpoint) {
    return cachedTokenEndpoint;
  }

  const res = await fetch(`${process.env.AUTH_COGNITO_ISSUER}/.well-known/openid-configuration`);
  if (!res.ok) {
    throw new Error(`OIDC discovery failed: ${res.status}`);
  }

  const meta = (await res.json()) as { token_endpoint: string };
  cachedTokenEndpoint = meta.token_endpoint;

  return cachedTokenEndpoint;
}

/**
 * refreshAccessToken rotates an expired Cognito access token using the stored
 * refresh token. Edge-safe (btoa/fetch/URLSearchParams only). On failure it
 * tags the token with an error so the UI can force a fresh sign-in rather than
 * looping on a dead token.
 */
async function refreshAccessToken(token: JWT): Promise<JWT> {
  if (!token.refreshToken) {
    return { ...token, error: "RefreshAccessTokenError" };
  }

  try {
    const endpoint = await cognitoTokenEndpoint();
    const basicAuth = btoa(`${process.env.AUTH_COGNITO_ID}:${process.env.AUTH_COGNITO_SECRET}`);

    const res = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        Authorization: `Basic ${basicAuth}`,
      },
      body: new URLSearchParams({
        grant_type: "refresh_token",
        client_id: process.env.AUTH_COGNITO_ID ?? "",
        refresh_token: token.refreshToken,
      }),
    });

    const refreshed = (await res.json()) as {
      access_token: string;
      expires_in: number;
      refresh_token?: string;
    };
    if (!res.ok) {
      throw new Error("refresh rejected");
    }

    return {
      ...token,
      accessToken: refreshed.access_token,
      accessTokenExpires: Math.floor(Date.now() / 1000) + refreshed.expires_in,
      // Cognito does not rotate the refresh token on refresh — keep ours.
      refreshToken: refreshed.refresh_token ?? token.refreshToken,
      error: undefined,
    };
  } catch {
    return { ...token, error: "RefreshAccessTokenError" };
  }
}

// Auth.js (NextAuth v5) configured against the Janus Cognito user pool. The
// portal is a confidential OAuth client: the code→token exchange happens
// server-side, and the resulting Cognito *access token* is what the
// license-server validates (token_use=access). We persist it into the session
// so the server-side BFF (lib/api.ts) can forward it as a Bearer token.
//
// Env: AUTH_SECRET, AUTH_COGNITO_ID, AUTH_COGNITO_SECRET, AUTH_COGNITO_ISSUER.
export const { handlers, signIn, signOut, auth } = NextAuth({
  providers: [
    Cognito({
      clientId: process.env.AUTH_COGNITO_ID,
      clientSecret: process.env.AUTH_COGNITO_SECRET,
      issuer: process.env.AUTH_COGNITO_ISSUER,
      // Cognito puts a `nonce` claim in the ID token even when none was
      // requested — not OIDC-compliant. If `nonce` isn't in our checks, Auth.js
      // doesn't expect it, and oauth4webapi rejects the token with
      // "unexpected ID Token nonce claim value" (long-standing: nextauthjs/
      // next-auth#7313). Including `nonce` makes Auth.js send one and validate
      // it, so Cognito echoing the value we sent is legal and matches.
      //
      // We deliberately omit `pkce`: Cognito's discovery doesn't advertise S256,
      // so Auth.js force-drops PKCE to a nonce-only flow at sign-in but still
      // expects a PKCE verifier at the callback — that authorize/callback
      // asymmetry is the actual break. The confidential client secret
      // authenticates the code exchange (what PKCE guards for public clients);
      // `state` + `nonce` cover CSRF/replay.
      checks: ["nonce", "state"],
    }),
  ],
  callbacks: {
    // jwt() runs whenever the session token is created/updated/read. On initial
    // sign-in `account` is present — capture the access token, its expiry, and
    // the refresh token. On later reads, return the token as-is while still
    // valid, otherwise rotate it via the refresh token. (Cognito access tokens
    // live ~1h; the session cookie lives much longer, so without this the BFF
    // would forward a dead token and the license-server 401s.)
    async jwt({ token, account }) {
      if (account) {
        token.accessToken = account.access_token;
        token.accessTokenExpires = account.expires_at; // epoch seconds
        token.refreshToken = account.refresh_token;
        token.error = undefined;

        return token;
      }

      // Still valid (60s skew) — use as-is.
      if (token.accessTokenExpires && Date.now() < (token.accessTokenExpires - 60) * 1000) {
        return token;
      }

      // Expired — transparently rotate using the refresh token.
      return refreshAccessToken(token);
    },
    // session() shapes what server code (and the client) sees. We surface the
    // (possibly just-refreshed) access token so the server-side BFF can attach
    // it, plus any refresh error so the UI can prompt a re-login.
    async session({ session, token }) {
      session.accessToken = token.accessToken;
      session.error = token.error;

      return session;
    },
  },
});
