# ADR-0006 Next.js 16 frontend, the Go API is the authority

Status: accepted

Decision: Next.js 16 App Router, React 19, TypeScript, Tailwind, shadcn/ui + Radix, ECharts, TanStack Query (server state), Zustand (UI state), React Hook Form + Zod. Next.js renders UI and holds the Auth0 session in an httpOnly cookie; browser calls go through /api/proxy, which attaches the access token server-side and exposes only GET/POST under /api/v1. All business logic, authorization and tenant isolation live in Go. TypeScript API types are generated from backend/api/openapi.yaml (npm run gen:api); responses are validated with zod at the boundary.
