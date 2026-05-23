'use client';

import { useEffect } from 'react';
import { useAuthStore } from '@/stores/useAuthStore';

export default function CallbackPage() {
  const { fetchMe, user } = useAuthStore();

  useEffect(() => {
    const poll = setInterval(async () => {
      await fetchMe();
    }, 500);
    fetchMe();
    return () => clearInterval(poll);
  }, [fetchMe]);

  useEffect(() => {
    if (user) {
      window.location.href = '/';
    }
  }, [user]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-sm text-[var(--text-muted)]">Signing in…</p>
    </div>
  );
}
