export interface User {
  id: number;
  email: string;
  name?: string;
  role: string;
}

export interface LoginResp {
  token: string;
  user: User;
}

export interface PageItem {
  id: number;
  pancake_page_id: string;
  platform: string;
  name: string;
  status: string;
  conversation_count: number;
}

export interface Conversation {
  id: number;
  pancake_conversation_id: string;
  type: string;
  snippet: string;
  message_count: number;
  last_message_at: string;
  updated_at: string;
}

export interface Customer {
  id: number;
  name: string;
  phone: string;
  email: string;
  fb_id: string;
  gender: string;
}

export interface Tag {
  id: number;
  name: string;
  color: string;
}

export interface ConversationDetail extends Conversation {
  customer: Customer | null;
  tags: Tag[];
}

export interface Message {
  id: number;
  pancake_message_id: string;
  sender_id: string;
  direction: "in" | "out" | string;
  type: string;
  content: string;
  attachments: unknown;
  inserted_at: string;
}

export interface Paged<T> {
  items: T[];
  next_cursor: string | null;
}

export interface DailyCount {
  day: string;
  count: number;
}

export interface PageCount {
  page_id: number;
  page_name: string;
  conversations: number;
  messages: number;
}

export interface Overview {
  pages: number;
  conversations: number;
  messages: number;
  by_day: DailyCount[];
  by_page: PageCount[];
}

export interface CrawlRun {
  id: number;
  started_at: string;
  finished_at: string | null;
  trigger: string;
  status: string;
  error: string;
}
