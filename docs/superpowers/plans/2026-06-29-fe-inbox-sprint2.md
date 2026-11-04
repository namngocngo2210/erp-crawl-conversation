# FE Sprint 2 — Khung SPA + Inbox — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dựng SPA React + Vite + TS với login JWT và màn Inbox 3-pane (pages → conversations infinite scroll → message thread bubble in/out + filter direction → customer panel), gọi read API của Sprint 1.

**Architecture:** SPA thuần trong `/web`. Layering FE: `features/* (UI)` → `hooks (TanStack Query)` → `services (axios client)` → `types`. Auth bằng JWT lưu localStorage, axios interceptor gắn token và xử lý 401. Routing type-safe bằng TanStack Router.

**Tech Stack:** React 19, Vite, TypeScript, TailwindCSS, shadcn/ui, lucide-react, @tanstack/react-query, @tanstack/react-router, axios, vitest + @testing-library/react.

## Global Constraints

- SPA thuần (CSR), không SSR. Thư mục gốc FE: `/web`.
- shadcn/ui cài qua CLI (không tự code component cơ bản); light theme tông xanh da trời; icons `lucide-react`.
- Không đặt business logic trong component UI; gọi API qua `services`; data fetching qua hooks TanStack Query.
- API base URL từ `VITE_API_BASE_URL`; dev dùng Vite proxy `/api` → `http://localhost:8080`.
- Response phân trang dạng `{ items, next_cursor }`; dùng `useInfiniteQuery`.
- File < ~300 dòng; tách theo trách nhiệm.
- Token lưu localStorage key `auth_token`; 401 → xoá token → redirect `/login`.
- Commit message tiếng Anh, ngắn gọn.

---

## File Structure

**Create (dưới `/web`):**
- `package.json`, `vite.config.ts`, `tsconfig.json`, `tsconfig.node.json`, `index.html`, `.env`, `.env.example`, `.gitignore`
- `tailwind.config.js`, `postcss.config.js`, `components.json` (shadcn), `src/index.css`
- `src/main.tsx` — bootstrap React + providers + router
- `src/lib/utils.ts` — `cn` helper (shadcn)
- `src/lib/format.ts` — format date/phone
- `src/types/api.ts` — type khớp DTO backend
- `src/services/client.ts` — axios instance + interceptor
- `src/services/auth.ts` — login API
- `src/services/conversations.ts` — pages/conversations/messages API
- `src/features/auth/token-store.ts` — get/set/clear token
- `src/features/auth/use-auth.ts` — hook auth state
- `src/features/auth/login-page.tsx` — màn login
- `src/features/inbox/use-pages.ts`, `use-conversations.ts`, `use-conversation.ts`, `use-messages.ts` — TanStack Query hooks
- `src/features/inbox/page-selector.tsx`, `conversation-list.tsx`, `message-thread.tsx`, `customer-panel.tsx`, `inbox-page.tsx`
- `src/app/router.tsx` — route tree + auth guard
- `src/app/root-layout.tsx` — layout + header + logout
- `src/components/ui/*` — shadcn components (cài qua CLI)
- Test: `src/lib/format.test.ts`, `src/features/auth/token-store.test.ts`, `src/features/inbox/message-thread.test.tsx`

**Modify:**
- `.gitignore` gốc repo — đảm bảo `web/node_modules`, `web/dist` được ignore.

---

## Task 1: Scaffold Vite + cấu hình nền

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/index.html`, `web/src/main.tsx`, `web/src/vite-env.d.ts`, `web/.env`, `web/.env.example`, `web/.gitignore`

- [ ] **Step 1: Scaffold project**

Run (từ thư mục gốc repo):
```bash
cd web 2>/dev/null || (npm create vite@latest web -- --template react-ts && cd web)
```
Nếu thư mục `web` chưa tồn tại, lệnh trên tạo mới. Sau scaffold, vào `web/`.

- [ ] **Step 2: Install dependencies**

Run trong `web/`:
```bash
npm install
npm install @tanstack/react-query @tanstack/react-router axios lucide-react clsx tailwind-merge
npm install -D tailwindcss postcss autoprefixer vitest @testing-library/react @testing-library/jest-dom jsdom @types/node
```

- [ ] **Step 3: Configure Vite (proxy + alias + vitest)**

Overwrite `web/vite.config.ts`:

```ts
/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
  },
});
```

Create `web/src/test-setup.ts`:
```ts
import "@testing-library/jest-dom";
```

- [ ] **Step 4: TS path alias**

In `web/tsconfig.json`, ensure `compilerOptions` has:
```json
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
```

- [ ] **Step 5: Env files**

Create `web/.env`:
```
VITE_API_BASE_URL=/api/v1
```
Create `web/.env.example`:
```
VITE_API_BASE_URL=/api/v1
```

- [ ] **Step 6: Verify dev build compiles**

Run: `npm run build`
Expected: build succeeds (Vite default app).

- [ ] **Step 7: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "chore: scaffold web spa (vite react ts)"
```

---

## Task 2: Tailwind + shadcn/ui (light blue theme)

**Files:**
- Create/Modify: `web/tailwind.config.js`, `web/postcss.config.js`, `web/src/index.css`, `web/components.json`, `web/src/lib/utils.ts`

- [ ] **Step 1: Init Tailwind**

Run trong `web/`:
```bash
npx tailwindcss init -p
```

- [ ] **Step 2: Configure Tailwind content + theme**

Overwrite `web/tailwind.config.js`:
```js
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: { DEFAULT: "hsl(var(--primary))", foreground: "hsl(var(--primary-foreground))" },
        muted: { DEFAULT: "hsl(var(--muted))", foreground: "hsl(var(--muted-foreground))" },
      },
      borderRadius: { lg: "var(--radius)", md: "calc(var(--radius) - 2px)" },
    },
  },
  plugins: [],
};
```

- [ ] **Step 3: Base CSS variables (sky-blue light theme)**

Overwrite `web/src/index.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --background: 0 0% 100%;
  --foreground: 222 47% 11%;
  --primary: 205 90% 48%;
  --primary-foreground: 0 0% 100%;
  --muted: 210 40% 96%;
  --muted-foreground: 215 16% 47%;
  --border: 214 32% 91%;
  --radius: 0.5rem;
}

* { border-color: hsl(var(--border)); }
body { background: hsl(var(--background)); color: hsl(var(--foreground)); }
```

- [ ] **Step 4: shadcn utils + init**

Create `web/src/lib/utils.ts`:
```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

Run shadcn init + add base components:
```bash
npx shadcn@latest init -d
npx shadcn@latest add button input card avatar badge scroll-area separator skeleton sonner
```
> Nếu CLI hỏi, chọn: style Default, base color Slate, CSS variables yes. Nếu CLI ghi đè `index.css`, giữ lại block `:root` sky-blue ở Step 3 (merge thủ công, giữ `--primary: 205 90% 48%`).

- [ ] **Step 5: Verify build**

Run: `npm run build`
Expected: success.

- [ ] **Step 6: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "chore: tailwind and shadcn ui with sky-blue theme"
```

---

## Task 3: Types + format utils (with tests)

**Files:**
- Create: `web/src/types/api.ts`, `web/src/lib/format.ts`
- Test: `web/src/lib/format.test.ts`

**Interfaces:**
- Produces (`types/api.ts`): `User`, `LoginResp`, `PageItem`, `Conversation`, `Customer`, `Tag`, `ConversationDetail`, `Message`, `Paged<T>` khớp DTO backend (snake_case fields).
- Produces (`format.ts`): `formatDateTime(iso: string): string`, `formatRelative(iso: string): string`.

- [ ] **Step 1: Write types**

Create `web/src/types/api.ts`:
```ts
export interface User { id: number; email: string; name?: string; role: string; }
export interface LoginResp { token: string; user: User; }

export interface PageItem {
  id: number; pancake_page_id: string; platform: string;
  name: string; status: string; conversation_count: number;
}

export interface Conversation {
  id: number; pancake_conversation_id: string; type: string;
  snippet: string; message_count: number;
  last_message_at: string; updated_at: string;
}

export interface Customer {
  id: number; name: string; phone: string; email: string; fb_id: string; gender: string;
}
export interface Tag { id: number; name: string; color: string; }

export interface ConversationDetail extends Conversation {
  customer: Customer | null;
  tags: Tag[];
}

export interface Message {
  id: number; pancake_message_id: string; sender_id: string;
  direction: "in" | "out" | string; type: string; content: string;
  attachments: unknown; inserted_at: string;
}

export interface Paged<T> { items: T[]; next_cursor: string | null; }
```

- [ ] **Step 2: Write the failing test**

Create `web/src/lib/format.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { formatDateTime } from "./format";

describe("formatDateTime", () => {
  it("formats an ISO string into a non-empty locale string", () => {
    const out = formatDateTime("2026-06-29T10:00:00Z");
    expect(out).toBeTruthy();
    expect(out).not.toBe("2026-06-29T10:00:00Z");
  });
  it("returns empty string for falsy input", () => {
    expect(formatDateTime("")).toBe("");
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `npm run test -- format` (add script if missing: `"test": "vitest run"`)
Expected: FAIL (cannot find ./format).

- [ ] **Step 4: Implement format utils**

Create `web/src/lib/format.ts`:
```ts
export function formatDateTime(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("vi-VN", { dateStyle: "short", timeStyle: "short" });
}

export function formatRelative(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso).getTime();
  if (Number.isNaN(d)) return iso;
  const diff = Date.now() - d;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "vừa xong";
  if (mins < 60) return `${mins} phút trước`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} giờ trước`;
  return `${Math.floor(hrs / 24)} ngày trước`;
}
```

Ensure `web/package.json` scripts include:
```json
    "test": "vitest run",
    "test:watch": "vitest"
```

- [ ] **Step 5: Run test to verify it passes**

Run: `npm run test -- format`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: api types and format utils"
```

---

## Task 4: Token store + axios client (with tests)

**Files:**
- Create: `web/src/features/auth/token-store.ts`, `web/src/services/client.ts`, `web/src/services/auth.ts`
- Test: `web/src/features/auth/token-store.test.ts`

**Interfaces:**
- Produces (`token-store.ts`): `getToken(): string|null`, `setToken(t: string): void`, `clearToken(): void`, const `TOKEN_KEY = "auth_token"`.
- Produces (`client.ts`): default export `api` (axios instance, baseURL `import.meta.env.VITE_API_BASE_URL`), request interceptor gắn Bearer, response interceptor: 401 → `clearToken()` + redirect `/login`.
- Produces (`auth.ts`): `login(email, password): Promise<LoginResp>`, `fetchMe(): Promise<User>`.

- [ ] **Step 1: Write the failing test**

Create `web/src/features/auth/token-store.test.ts`:
```ts
import { describe, it, expect, beforeEach } from "vitest";
import { getToken, setToken, clearToken } from "./token-store";

describe("token-store", () => {
  beforeEach(() => localStorage.clear());
  it("round trips a token", () => {
    expect(getToken()).toBeNull();
    setToken("abc");
    expect(getToken()).toBe("abc");
    clearToken();
    expect(getToken()).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm run test -- token-store`
Expected: FAIL.

- [ ] **Step 3: Implement token store**

Create `web/src/features/auth/token-store.ts`:
```ts
export const TOKEN_KEY = "auth_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}
export function setToken(t: string): void {
  localStorage.setItem(TOKEN_KEY, t);
}
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm run test -- token-store`
Expected: PASS

- [ ] **Step 5: Implement axios client + auth service**

Create `web/src/services/client.ts`:
```ts
import axios from "axios";
import { getToken, clearToken } from "@/features/auth/token-store";

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? "/api/v1",
});

api.interceptors.request.use((config) => {
  const token = getToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err?.response?.status === 401) {
      clearToken();
      if (window.location.pathname !== "/login") {
        window.location.href = "/login";
      }
    }
    return Promise.reject(err);
  }
);

export default api;
```

Create `web/src/services/auth.ts`:
```ts
import api from "./client";
import type { LoginResp, User } from "@/types/api";

export async function login(email: string, password: string): Promise<LoginResp> {
  const { data } = await api.post<LoginResp>("/auth/login", { email, password });
  return data;
}

export async function fetchMe(): Promise<User> {
  const { data } = await api.get<User>("/auth/me");
  return data;
}
```

- [ ] **Step 6: Run build**

Run: `npm run build`
Expected: success.

- [ ] **Step 7: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: token store and axios client with auth interceptor"
```

---

## Task 5: Conversations service + Query hooks

**Files:**
- Create: `web/src/services/conversations.ts`, `web/src/features/inbox/use-pages.ts`, `use-conversations.ts`, `use-conversation.ts`, `use-messages.ts`

**Interfaces:**
- Produces (`conversations.ts`):
  - `getPages(): Promise<{items: PageItem[]}>`
  - `getConversations(params: {page_id: number; cursor?: string; limit?: number}): Promise<Paged<Conversation>>`
  - `getConversation(id: number): Promise<ConversationDetail>`
  - `getMessages(params: {conversation_id: number; direction?: string; cursor?: string; limit?: number}): Promise<Paged<Message>>`
- Produces hooks: `usePages()`, `useConversations(pageId)` (`useInfiniteQuery`), `useConversation(id)`, `useMessages(convId, direction)` (`useInfiniteQuery`).

- [ ] **Step 1: Implement service**

Create `web/src/services/conversations.ts`:
```ts
import api from "./client";
import type { PageItem, Conversation, ConversationDetail, Message, Paged } from "@/types/api";

export async function getPages(): Promise<{ items: PageItem[] }> {
  const { data } = await api.get<{ items: PageItem[] }>("/pages");
  return data;
}

export async function getConversations(params: {
  page_id: number; cursor?: string; limit?: number;
}): Promise<Paged<Conversation>> {
  const { data } = await api.get<Paged<Conversation>>("/conversations", { params });
  return data;
}

export async function getConversation(id: number): Promise<ConversationDetail> {
  const { data } = await api.get<ConversationDetail>(`/conversations/${id}`);
  return data;
}

export async function getMessages(params: {
  conversation_id: number; direction?: string; cursor?: string; limit?: number;
}): Promise<Paged<Message>> {
  const { conversation_id, ...rest } = params;
  const { data } = await api.get<Paged<Message>>(`/conversations/${conversation_id}/messages`, { params: rest });
  return data;
}
```

- [ ] **Step 2: Implement hooks**

Create `web/src/features/inbox/use-pages.ts`:
```ts
import { useQuery } from "@tanstack/react-query";
import { getPages } from "@/services/conversations";

export function usePages() {
  return useQuery({ queryKey: ["pages"], queryFn: getPages });
}
```

Create `web/src/features/inbox/use-conversations.ts`:
```ts
import { useInfiniteQuery } from "@tanstack/react-query";
import { getConversations } from "@/services/conversations";

export function useConversations(pageId: number | null) {
  return useInfiniteQuery({
    queryKey: ["conversations", pageId],
    enabled: pageId != null,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      getConversations({ page_id: pageId as number, cursor: pageParam, limit: 30 }),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
```

Create `web/src/features/inbox/use-conversation.ts`:
```ts
import { useQuery } from "@tanstack/react-query";
import { getConversation } from "@/services/conversations";

export function useConversation(id: number | null) {
  return useQuery({
    queryKey: ["conversation", id],
    enabled: id != null,
    queryFn: () => getConversation(id as number),
  });
}
```

Create `web/src/features/inbox/use-messages.ts`:
```ts
import { useInfiniteQuery } from "@tanstack/react-query";
import { getMessages } from "@/services/conversations";

export function useMessages(convId: number | null, direction: string) {
  return useInfiniteQuery({
    queryKey: ["messages", convId, direction],
    enabled: convId != null,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      getMessages({
        conversation_id: convId as number,
        direction: direction || undefined,
        cursor: pageParam,
        limit: 30,
      }),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
```

- [ ] **Step 3: Run build**

Run: `npm run build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: conversations service and query hooks"
```

---

## Task 6: Login page + auth hook + router guard

**Files:**
- Create: `web/src/features/auth/use-auth.ts`, `web/src/features/auth/login-page.tsx`, `web/src/app/root-layout.tsx`, `web/src/app/router.tsx`, `web/src/main.tsx` (overwrite)

**Interfaces:**
- Consumes: `login`, `fetchMe`, token store, shadcn `button`/`input`/`card`.
- Produces:
  - `use-auth.ts`: `useAuth()` → `{ isAuthenticated: boolean }` (dựa trên token tồn tại); `useLogin()` mutation.
  - `router.tsx`: route tree `/login` (public) + `/` (guarded → inbox). Guard: chưa có token → redirect `/login`.

- [ ] **Step 1: Auth hook**

Create `web/src/features/auth/use-auth.ts`:
```ts
import { useMutation } from "@tanstack/react-query";
import { login } from "@/services/auth";
import { setToken, getToken, clearToken } from "./token-store";

export function useAuth() {
  return { isAuthenticated: !!getToken(), logout: clearToken };
}

export function useLogin() {
  return useMutation({
    mutationFn: (vars: { email: string; password: string }) =>
      login(vars.email, vars.password),
    onSuccess: (data) => setToken(data.token),
  });
}
```

- [ ] **Step 2: Login page**

Create `web/src/features/auth/login-page.tsx`:
```tsx
import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useLogin } from "./use-auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";

export function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const login = useLogin();
  const navigate = useNavigate();

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    login.mutate(
      { email, password },
      { onSuccess: () => navigate({ to: "/" }) }
    );
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted">
      <Card className="w-full max-w-sm p-6">
        <h1 className="mb-4 text-xl font-semibold text-primary">Đăng nhập</h1>
        <form onSubmit={onSubmit} className="space-y-3">
          <Input type="email" placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          <Input type="password" placeholder="Mật khẩu" value={password} onChange={(e) => setPassword(e.target.value)} required />
          {login.isError && <p className="text-sm text-red-600">Sai email hoặc mật khẩu</p>}
          <Button type="submit" className="w-full" disabled={login.isPending}>
            {login.isPending ? "Đang đăng nhập..." : "Đăng nhập"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
```

- [ ] **Step 3: Root layout**

Create `web/src/app/root-layout.tsx`:
```tsx
import { Outlet, useNavigate } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/features/auth/use-auth";
import { LogOut, MessagesSquare } from "lucide-react";

export function RootLayout() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  return (
    <div className="flex h-screen flex-col">
      <header className="flex items-center justify-between border-b px-4 py-2">
        <div className="flex items-center gap-2 font-semibold text-primary">
          <MessagesSquare className="h-5 w-5" /> Pancake Inbox
        </div>
        <Button variant="ghost" size="sm" onClick={() => { logout(); navigate({ to: "/login" }); }}>
          <LogOut className="mr-1 h-4 w-4" /> Đăng xuất
        </Button>
      </header>
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 4: Router with guard**

Create `web/src/app/router.tsx`:
```tsx
import {
  createRootRoute, createRoute, createRouter, redirect, Outlet,
} from "@tanstack/react-router";
import { getToken } from "@/features/auth/token-store";
import { LoginPage } from "@/features/auth/login-page";
import { RootLayout } from "./root-layout";
import { InboxPage } from "@/features/inbox/inbox-page";

const rootRoute = createRootRoute({ component: Outlet });

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  component: RootLayout,
  beforeLoad: () => {
    if (!getToken()) throw redirect({ to: "/login" });
  },
});

const inboxRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/",
  component: InboxPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([inboxRoute]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register { router: typeof router; }
}
```

- [ ] **Step 5: Bootstrap main.tsx**

Overwrite `web/src/main.tsx`:
```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { router } from "./app/router";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </React.StrictMode>
);
```

> Implementer note: `InboxPage` chưa tồn tại tới Task 7. Để build pass tạm thời, tạo stub `web/src/features/inbox/inbox-page.tsx` export `export function InboxPage() { return null; }` rồi thay ở Task 7. Commit stub cùng task này.

- [ ] **Step 6: Build**

Run: `npm run build`
Expected: success (với stub InboxPage).

- [ ] **Step 7: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: login page, auth hook and guarded router"
```

---

## Task 7: Inbox 3-pane (page selector, conversation list, message thread, customer panel)

**Files:**
- Create/overwrite: `web/src/features/inbox/page-selector.tsx`, `conversation-list.tsx`, `message-thread.tsx`, `customer-panel.tsx`, `inbox-page.tsx`
- Test: `web/src/features/inbox/message-thread.test.tsx`

**Interfaces:**
- Consumes: hooks Task 5, shadcn components, `formatDateTime/formatRelative`, lucide icons.
- `InboxPage` quản state: `pageId`, `convId`, `direction`. Layout 3 cột (page+conv list | thread | customer).

- [ ] **Step 1: Page selector**

Create `web/src/features/inbox/page-selector.tsx`:
```tsx
import { usePages } from "./use-pages";

export function PageSelector({ value, onChange }: { value: number | null; onChange: (id: number) => void }) {
  const { data, isLoading } = usePages();
  if (isLoading) return <div className="p-2 text-sm text-muted-foreground">Đang tải pages...</div>;
  return (
    <select
      className="w-full rounded-md border px-2 py-1.5 text-sm"
      value={value ?? ""}
      onChange={(e) => onChange(Number(e.target.value))}
    >
      <option value="" disabled>Chọn page…</option>
      {data?.items.map((p) => (
        <option key={p.id} value={p.id}>{p.name} ({p.conversation_count})</option>
      ))}
    </select>
  );
}
```

- [ ] **Step 2: Conversation list (infinite scroll)**

Create `web/src/features/inbox/conversation-list.tsx`:
```tsx
import { useConversations } from "./use-conversations";
import { formatRelative } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export function ConversationList({
  pageId, selectedId, onSelect,
}: { pageId: number | null; selectedId: number | null; onSelect: (id: number) => void }) {
  const q = useConversations(pageId);
  if (pageId == null) return <div className="p-3 text-sm text-muted-foreground">Chọn page để xem hội thoại.</div>;
  if (q.isLoading) return <div className="p-3 text-sm text-muted-foreground">Đang tải…</div>;
  const items = q.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <div className="flex h-full flex-col overflow-y-auto">
      {items.map((c) => (
        <button
          key={c.id}
          onClick={() => onSelect(c.id)}
          className={cn("border-b px-3 py-2 text-left hover:bg-muted", selectedId === c.id && "bg-muted")}
        >
          <div className="flex justify-between">
            <span className="truncate text-sm font-medium">#{c.pancake_conversation_id}</span>
            <span className="text-xs text-muted-foreground">{formatRelative(c.last_message_at)}</span>
          </div>
          <p className="truncate text-xs text-muted-foreground">{c.snippet}</p>
        </button>
      ))}
      {q.hasNextPage && (
        <Button variant="ghost" size="sm" onClick={() => q.fetchNextPage()} disabled={q.isFetchingNextPage}>
          {q.isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
        </Button>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Message thread (bubble in/out + direction filter)**

Create `web/src/features/inbox/message-thread.tsx`:
```tsx
import { useMessages } from "./use-messages";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import type { Message } from "@/types/api";

const FILTERS = [
  { key: "", label: "Tất cả" },
  { key: "in", label: "Khách gửi" },
  { key: "out", label: "Trang gửi" },
];

export function MessageThread({
  convId, direction, onDirectionChange,
}: { convId: number | null; direction: string; onDirectionChange: (d: string) => void }) {
  const q = useMessages(convId, direction);
  if (convId == null) return <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Chọn một hội thoại.</div>;
  const items: Message[] = q.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <div className="flex h-full flex-col">
      <div className="flex gap-1 border-b p-2">
        {FILTERS.map((f) => (
          <Button key={f.key} size="sm" variant={direction === f.key ? "default" : "ghost"} onClick={() => onDirectionChange(f.key)}>
            {f.label}
          </Button>
        ))}
      </div>
      <div className="flex flex-1 flex-col-reverse overflow-y-auto p-3">
        {items.map((m) => (
          <div key={m.id} className={cn("mb-2 flex", m.direction === "out" ? "justify-end" : "justify-start")}>
            <div className={cn("max-w-[70%] rounded-lg px-3 py-2 text-sm",
              m.direction === "out" ? "bg-primary text-primary-foreground" : "bg-muted")}>
              <p className="whitespace-pre-wrap break-words">{m.content}</p>
              <span className="mt-1 block text-[10px] opacity-70">{formatDateTime(m.inserted_at)}</span>
            </div>
          </div>
        ))}
        {q.hasNextPage && (
          <Button variant="ghost" size="sm" onClick={() => q.fetchNextPage()} disabled={q.isFetchingNextPage}>
            {q.isFetchingNextPage ? "Đang tải…" : "Tải tin cũ hơn"}
          </Button>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Customer panel**

Create `web/src/features/inbox/customer-panel.tsx`:
```tsx
import { useConversation } from "./use-conversation";
import { Badge } from "@/components/ui/badge";
import { User, Phone, Mail } from "lucide-react";

export function CustomerPanel({ convId }: { convId: number | null }) {
  const { data } = useConversation(convId);
  if (convId == null || !data) return <div className="p-3 text-sm text-muted-foreground">—</div>;
  const c = data.customer;
  return (
    <div className="space-y-3 p-3 text-sm">
      <div className="flex items-center gap-2 font-medium"><User className="h-4 w-4" />{c?.name || "Khách ẩn danh"}</div>
      {c?.phone && <div className="flex items-center gap-2 text-muted-foreground"><Phone className="h-4 w-4" />{c.phone}</div>}
      {c?.email && <div className="flex items-center gap-2 text-muted-foreground"><Mail className="h-4 w-4" />{c.email}</div>}
      <div className="flex flex-wrap gap-1">
        {data.tags.map((t) => (
          <Badge key={t.id} style={{ backgroundColor: t.color || undefined }}>{t.name}</Badge>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Inbox page (compose 3-pane)**

Overwrite `web/src/features/inbox/inbox-page.tsx`:
```tsx
import { useState } from "react";
import { PageSelector } from "./page-selector";
import { ConversationList } from "./conversation-list";
import { MessageThread } from "./message-thread";
import { CustomerPanel } from "./customer-panel";

export function InboxPage() {
  const [pageId, setPageId] = useState<number | null>(null);
  const [convId, setConvId] = useState<number | null>(null);
  const [direction, setDirection] = useState("");

  return (
    <div className="grid h-full grid-cols-[300px_1fr_280px]">
      <aside className="flex flex-col border-r">
        <div className="border-b p-2"><PageSelector value={pageId} onChange={(id) => { setPageId(id); setConvId(null); }} /></div>
        <ConversationList pageId={pageId} selectedId={convId} onSelect={setConvId} />
      </aside>
      <section className="overflow-hidden">
        <MessageThread convId={convId} direction={direction} onDirectionChange={setDirection} />
      </section>
      <aside className="border-l"><CustomerPanel convId={convId} /></aside>
    </div>
  );
}
```

- [ ] **Step 6: Smoke test message thread**

Create `web/src/features/inbox/message-thread.test.tsx`:
```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageThread } from "./message-thread";

vi.mock("./use-messages", () => ({
  useMessages: () => ({
    data: { pages: [{ items: [
      { id: 1, direction: "in", content: "Xin chào", inserted_at: "2026-06-29T10:00:00Z" },
      { id: 2, direction: "out", content: "Chào bạn", inserted_at: "2026-06-29T10:01:00Z" },
    ], next_cursor: null }] },
    hasNextPage: false, isFetchingNextPage: false, fetchNextPage: vi.fn(), isLoading: false,
  }),
}));

describe("MessageThread", () => {
  beforeEach(() => {});
  it("renders both inbound and outbound messages", () => {
    render(<MessageThread convId={1} direction="" onDirectionChange={() => {}} />);
    expect(screen.getByText("Xin chào")).toBeInTheDocument();
    expect(screen.getByText("Chào bạn")).toBeInTheDocument();
  });
});
```

- [ ] **Step 7: Run tests + build**

Run: `npm run test && npm run build`
Expected: PASS + build success.

- [ ] **Step 8: Commit**

```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: inbox 3-pane with conversation list, thread and customer panel"
```

---

## Task 8: Manual smoke + README + progress

**Files:**
- Modify: `README.md`, `docs/superpowers/specs/2026-06-29-read-api-fe-spa-design.md`

- [ ] **Step 1: Document FE run**

Add to `README.md`:
```markdown
## Frontend (web/)
- `cd web && npm install && npm run dev` → http://localhost:5173 (proxy `/api` → :8080).
- Đăng nhập bằng `ADMIN_EMAIL`/`ADMIN_PASSWORD` đã seed ở backend.
- Build production: `npm run build` (output `web/dist`).
```

- [ ] **Step 2: Mark Sprint 2 progress in spec**

Đổi `### Sprint 2 — FE khung + Inbox (tiến độ: ☐)` → `(tiến độ: ✅)`, tick các box, note "đã làm / chưa làm".

- [ ] **Step 3: Commit**

```bash
git add README.md docs/
git commit -m "docs: frontend run guide and sprint2 progress"
```

---

## Self-Review notes (đã kiểm)
- **Coverage Sprint 2:** scaffold ✅(T1), tailwind/shadcn theme xanh ✅(T2), types/utils ✅(T3), token+axios 401 ✅(T4), service+hooks infinite scroll ✅(T5), login+guard router ✅(T6), inbox 3-pane bubble in/out + filter direction ✅(T7).
- **Type consistency:** `Paged<T>`, `Conversation`, `Message`, `ConversationDetail` (T3) dùng nhất quán ở service (T5) và component (T7). `getToken/setToken/clearToken` (T4) dùng ở client + use-auth + router guard.
- **Phụ thuộc thứ tự:** T6 cần stub `InboxPage`, thay thật ở T7 — đã ghi rõ.
- **Placeholder scan:** không có TODO/TBD; "Implementer note" chỉ là hướng dẫn thao tác.
