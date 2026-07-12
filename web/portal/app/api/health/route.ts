// Liveness/readiness endpoint for the container orchestrator. Intentionally
// dependency-free: it reports that the Next.js server process is up and serving,
// not that downstream systems (the license-server) are reachable — a failing
// dependency should surface on the page, not flap the pod.
export const dynamic = "force-dynamic";

export function GET() {
  return Response.json({ status: "ok" }, { status: 200 });
}
