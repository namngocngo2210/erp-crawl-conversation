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
