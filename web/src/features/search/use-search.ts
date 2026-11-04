import { useInfiniteQuery } from "@tanstack/react-query";
import { searchMessages, type MsgSearchParams } from "@/services/search";

export function useSearchMessages(params: MsgSearchParams) {
  return useInfiniteQuery({
    queryKey: ["search-messages", params],
    enabled: !!params.q,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => searchMessages({ ...params, cursor: pageParam, limit: 30 }),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
