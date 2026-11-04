import api from "./client";
import type { PageItem, Conversation, ConversationDetail, Message, Paged } from "@/types/api";

export async function getPages(): Promise<{ items: PageItem[] }> {
  const { data } = await api.get<{ items: PageItem[] }>("/pages");
  return data;
}

export async function getConversations(params: {
  page_id: number; cursor?: string; limit?: number;
}): Promise<Paged<Conversation>> {
  const { data } = await api.get<Paged<Conversation>>("/conversations", { params });
  return data;
}

export async function getConversation(id: number): Promise<ConversationDetail> {
  const { data } = await api.get<ConversationDetail>(`/conversations/${id}`);
  return data;
}

export async function getMessages(params: {
  conversation_id: number; direction?: string; cursor?: string; limit?: number;
}): Promise<Paged<Message>> {
  const { conversation_id, ...rest } = params;
  const { data } = await api.get<Paged<Message>>(`/conversations/${conversation_id}/messages`, { params: rest });
  return data;
}
