import { useQuery } from "@tanstack/react-query";
import { getOverview } from "@/services/stats";

export function useOverview(days = 30) {
  return useQuery({ queryKey: ["overview", days], queryFn: () => getOverview(days) });
}
