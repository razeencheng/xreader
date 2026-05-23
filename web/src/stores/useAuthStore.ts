'use client';

import { create } from 'zustand';
import { apiFetch } from '@/lib/api-client';

export interface User {
  id: number;
  github_username: string;
  role: string;
  native_language: string;
  density_pref: string;
  theme_pref: string;
}

interface AuthState {
  user: User | null;
  isLoading: boolean;
  fetchMe: () => Promise<void>;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  fetchMe: async () => {
    try {
      const user = await apiFetch<User>('/api/auth/me');
      set({ user, isLoading: false });
    } catch {
      set({ user: null, isLoading: false });
    }
  },
  logout: async () => {
    try {
      await apiFetch('/api/auth/logout', { method: 'POST' });
    } finally {
      set({ user: null });
      window.location.href = '/login';
    }
  },
}));

export const useIsGuest = () => useAuthStore((s) => s.user?.role === 'guest');
