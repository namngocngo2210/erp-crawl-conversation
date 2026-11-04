import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DashboardPage } from "./dashboard-page";

vi.mock("./use-stats", () => ({
  useOverview: () => ({ data: { pages: 2, conversations: 5, messages: 99, by_day: [], by_page: [] } }),
}));
vi.mock("./use-crawl-runs", () => ({
  useCrawlRuns: () => ({ data: { items: [] } }),
}));

describe("DashboardPage", () => {
  it("renders totals", () => {
    render(<DashboardPage />);
    expect(screen.getByText("99")).toBeInTheDocument();
  });
});
