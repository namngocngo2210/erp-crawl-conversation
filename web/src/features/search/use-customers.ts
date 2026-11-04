import { useInfiniteQuery } from "@tanstack/react-query";
import { searchCustomers, type CustomerSearchParams } from "@/services/search";

export function useCustomers(params: CustomerSearchParams) {
  return useInfiniteQuery({
    queryKey: ["customers", params],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => searchCustomers({ ...params, cursor: pageParam, limit: 30 }),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
