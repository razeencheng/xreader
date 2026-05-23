# xReader Web Frontend

Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 frontend for xReader.

## Development

```bash
pnpm install
pnpm dev          # Dev server on :3000 (Turbopack)
pnpm build        # Production static export → out/
pnpm lint         # ESLint + TypeScript check
pnpm vitest run   # Unit tests
```

## Architecture

- **State:** Zustand (client UI prefs) + TanStack React Query (server data)
- **Styling:** Tailwind CSS 4 with CSS custom properties for theming
- **i18n:** 9 languages, client-side dictionary in `src/lib/i18n.ts`
- **Testing:** Vitest + Testing Library + MSW

## Directory Structure

```
src/
├── app/          # Next.js App Router pages
├── components/   # UI components by domain (feed/, reader/, layout/, etc.)
├── hooks/        # Custom React hooks
├── lib/          # Utilities (api-client, i18n, state cache)
└── stores/       # Zustand stores
```

The frontend is compiled to a static export and embedded in the Go binary for single-binary deployment.
