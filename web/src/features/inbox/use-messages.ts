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
