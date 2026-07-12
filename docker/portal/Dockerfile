# syntax=docker/dockerfile:1

# Production image for the Janus management portal (Next.js App Router).
#
# Build context is web/portal/ (a self-contained pnpm project) — NOT the repo
# root. Build it via:
#   docker build -f docker/portal/Dockerfile web/portal
# or the portal-image.yml workflow (context: web/portal).
#
# Uses Next's "standalone" output (next.config.ts → output: "standalone"), which
# emits a minimal self-contained server bundle. The final image carries only the
# Node runtime + that bundle + static assets — no pnpm, no dev deps, no source.

# ── deps: install with a warm pnpm store layer ────────────────────────────────
FROM node:22-bookworm-slim AS deps
ENV PNPM_HOME=/pnpm PATH=/pnpm:$PATH
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN --mount=type=cache,target=/pnpm/store pnpm install --frozen-lockfile

# ── builder: compile the Next standalone bundle ───────────────────────────────
FROM node:22-bookworm-slim AS builder
ENV PNPM_HOME=/pnpm PATH=/pnpm:$PATH NEXT_TELEMETRY_DISABLED=1
RUN corepack enable
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

# ── runner: minimal runtime ───────────────────────────────────────────────────
FROM node:22-bookworm-slim AS runner
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
WORKDIR /app

# The node image ships a non-root `node` user (uid 1000); run as it.
COPY --from=builder /app/public ./public
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static

USER node
EXPOSE 3000

# server.js is emitted at the standalone root by Next.
CMD ["node", "server.js"]
