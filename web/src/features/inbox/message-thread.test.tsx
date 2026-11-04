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
