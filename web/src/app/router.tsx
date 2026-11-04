import {
  createRootRoute, createRoute, createRouter, redirect, Outlet,
} from "@tanstack/react-router";
import { getToken } from "@/features/auth/token-store";
import { LoginPage } from "@/features/auth/login-page";
import { RootLayout } from "./root-layout";
import { InboxPage } from "@/features/inbox/inbox-page";
import { SearchPage } from "@/features/search/search-page";
import { DashboardPage } from "@/features/dashboard/dashboard-page";

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

const searchRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/search",
  component: SearchPage,
});

const dashboardRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/dashboard",
  component: DashboardPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([inboxRoute, searchRoute, dashboardRoute]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register { router: typeof router; }
}
