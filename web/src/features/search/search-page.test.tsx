import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { SearchPage } from "./search-page";

vi.mock("./use-search", () => ({
  useSearchMessages: () => ({
    data: { pages: [] }, isLoading: false, hasNextPage: false,
    isFetchingNextPage: false, fetchNextPage: vi.fn(),
  }),
}));

describe("SearchPage", () => {
  it("renders the search input", () => {
    render(<SearchPage />);
    expect(screen.getByPlaceholderText("Tìm nội dung tin nhắn…")).toBeInTheDocument();
  });
});
