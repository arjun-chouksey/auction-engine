import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import { api } from "../lib/api";
import type { User } from "../lib/types";

interface AuthValue {
  token: string | null; user: User | null; login(email: string, password: string): Promise<void>; logout(): void;
}
const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const value = useMemo<AuthValue>(() => ({
    token, user,
    async login(email, password) {
      const result = await api<{ access_token: string; user: User }>("/auth/login", undefined, { method: "POST", body: JSON.stringify({ email, password }) });
      setToken(result.access_token); setUser(result.user);
    },
    logout() { setToken(null); setUser(null); },
  }), [token, user]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
