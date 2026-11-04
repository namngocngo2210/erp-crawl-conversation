import { useMutation } from "@tanstack/react-query";
import { login } from "@/services/auth";
import { setToken, getToken, clearToken } from "./token-store";

export function useAuth() {
  return { isAuthenticated: !!getToken(), logout: clearToken };
}

export function useLogin() {
  return useMutation({
    mutationFn: (vars: { email: string; password: string }) =>
      login(vars.email, vars.password),
    onSuccess: (data) => setToken(data.token),
  });
}
