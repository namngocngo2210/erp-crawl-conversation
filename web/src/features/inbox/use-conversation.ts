import { useQuery } from "@tanstack/react-query";
import { getConversation } from "@/services/conversations";

export function useConversation(id: number | null) {
  return useQuery({
    queryKey: ["conversation", id],
    enabled: id != null,
    queryFn: () => getConversation(id as number),
  });
}
