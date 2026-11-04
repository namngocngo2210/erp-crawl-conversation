import api from "./client";
import type { Message, Customer, Paged } from "@/types/api";

export interface MsgSearchParams {
  q: string;
  page_id?: number;
  direction?: string;
  from?: string;
  to?: string;
  cursor?: string;
  limit?: number;
}

export async function searchMessages(p: MsgSearchParams): Promise<Paged<Message>> {
  const { data } = await api.get<Paged<Message>>("/search/messages", { params: p });
  return data;
}

export interface CustomerSearchParams {
  page_id?: number;
  q?: string;
  cursor?: string;
  limit?: number;
}

export async function searchCustomers(p: CustomerSearchParams): Promise<Paged<Customer>> {
  const { data } = await api.get<Paged<Customer>>("/customers", { params: p });
  return data;
}
