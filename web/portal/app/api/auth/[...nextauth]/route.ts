// Auth.js mounts its OAuth endpoints (sign-in, callback, session, sign-out) at
// /api/auth/* via this catch-all route handler.
import { handlers } from "@/auth";

export const { GET, POST } = handlers;
