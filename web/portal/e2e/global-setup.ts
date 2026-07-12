import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { encode } from "next-auth/jwt";

// Mints an Auth.js session cookie and writes it as a Playwright storageState, so
// every spec runs as an authenticated customer-admin without driving the real
// Cognito hosted UI. The cookie is a genuine encrypted Auth.js session JWT —
// signed with the same AUTH_SECRET the portal uses — so the middleware and the
// session callback accept it exactly as if the user had logged in. The embedded
// accessToken is what the BFF forwards to the (mocked) license-server.
//
// Over http://localhost the Auth.js cookie is non-secure, named
// `authjs.session-token`; the salt passed to encode() MUST equal that name.

export const STORAGE_STATE = resolve(__dirname, ".auth/state.json");
const COOKIE_NAME = "authjs.session-token";

export default async function globalSetup() {
  const secret = process.env.AUTH_SECRET;
  if (!secret) {
    throw new Error("AUTH_SECRET must be set for the e2e session fixture");
  }

  const token = {
    name: "Acme Admin",
    email: "admin@acme.test",
    sub: "e2e-admin-sub",
    accessToken: "e2e-access-token",
  };

  const value = await encode({ token, secret, salt: COOKIE_NAME, maxAge: 60 * 60 });

  const expires = Math.floor(Date.now() / 1000) + 60 * 60;
  const storageState = {
    cookies: [
      {
        name: COOKIE_NAME,
        value,
        domain: "localhost",
        path: "/",
        httpOnly: true,
        secure: false,
        sameSite: "Lax" as const,
        expires,
      },
    ],
    origins: [],
  };

  mkdirSync(dirname(STORAGE_STATE), { recursive: true });
  writeFileSync(STORAGE_STATE, JSON.stringify(storageState, null, 2));
}
