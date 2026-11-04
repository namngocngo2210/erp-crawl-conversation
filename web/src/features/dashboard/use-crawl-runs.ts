import { useQuery } from "@tanstack/react-query";
import { getCrawlRuns } from "@/services/stats";

export function useCrawlRuns(limit = 20) {
  return useQuery({ queryKey: ["crawl-runs", limit], queryFn: () => getCrawlRuns(limit) });
}
