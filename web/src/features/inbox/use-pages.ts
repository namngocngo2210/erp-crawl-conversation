import { useQuery } from "@tanstack/react-query";
import { getPages } from "@/services/conversations";

export function usePages() {
  return useQuery({ queryKey: ["pages"], queryFn: getPages });
}
