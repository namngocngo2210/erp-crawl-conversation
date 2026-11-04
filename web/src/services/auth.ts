import api from "./client";
import type { LoginResp, User } from "@/types/api";

export async function login(email: string, password: string): Promise<LoginResp> {
  const { data } = await api.post<LoginResp>("/auth/login", { email, password });
  return data;
}

export async function fetchMe(): Promise<User> {
  const { data } = await api.get<User>("/auth/me");
  return data;
}
