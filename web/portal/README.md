# Janus Management Portal (web/portal)

Next.js front-end over the license-server API. Greenfield as of B1 — this commit is
the project scaffold + tooling only; auth (B2), screens (B3), and the Playwright
suite (B4) build on top.

## Stack

| Concern        | Choice                                            |
| -------------- | ------------------------------------------------- |
| Framework      | Next.js 15 (**App Router**) + React 19            |
| Language       | TypeScript (strict)                               |
| Styling        | Tailwind CSS + the `cn()` helper (shadcn/ui style)|
| Package manager| pnpm                                              |
| Unit/component | Vitest + React Testing Library (jsdom)            |
| End-to-end     | Playwright (hermetic — MSW-mocked API, see B4)    |
| Lint/format    | ESLint (`next/core-web-vitals`) + Prettier        |

## Layout

```
web/portal/
├── app/                 # App Router: file-based routes
│   ├── layout.tsx       # root layout — wraps every page (html/body shell)
│   ├── page.tsx         # the "/" route (landing)
│   ├── page.test.tsx    # component test (Vitest)
│   └── globals.css      # Tailwind entry
├── lib/utils.ts         # cn() class-name helper
├── e2e/                 # Playwright specs
├── *.config.{ts,mjs}    # next / tailwind / postcss / vitest / playwright
└── package.json
```

A note on Next concepts for reviewers:

- **App Router** — routes are folders under `app/`. A folder's `page.tsx` is its
  page; `layout.tsx` wraps it. There's no router config file.
- **Server vs Client Components** — components are server-rendered by default (no
  JS shipped). A file opts into the browser with `"use client"` at the top (needed
  for state, effects, event handlers). B3 screens will use both.

## Commands

```bash
pnpm install        # install deps
pnpm dev            # local dev server (http://localhost:3000)
pnpm build          # production build (standalone output for Docker)
pnpm lint           # ESLint
pnpm typecheck      # tsc --noEmit
pnpm test           # Vitest (component/unit)
pnpm e2e            # Playwright (requires `pnpm exec playwright install` once)
```

These are also exposed as `mage web:*` targets in Part C.
