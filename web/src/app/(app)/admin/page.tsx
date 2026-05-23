'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/stores/useAuthStore';
import { useI18n } from '@/lib/i18n';
import {
  useAddAllowlistEntry,
  useAllowlist,
  useRemoveAllowlistEntry,
} from '@/lib/queries/admin';

function formatAddedAt(addedAt?: string) {
  if (!addedAt) return '—';
  const date = new Date(addedAt);
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString();
}

export default function AdminPage() {
  const { t } = useI18n();
  const router = useRouter();
  const user = useAuthStore((state) => state.user);
  const { data: entries, isLoading } = useAllowlist();
  const addEntry = useAddAllowlistEntry();
  const removeEntry = useRemoveAllowlistEntry();
  const [newUsername, setNewUsername] = useState('');

  useEffect(() => {
    if (user && user.role !== 'admin') {
      router.replace('/');
    }
  }, [router, user]);

  if (!user || user.role !== 'admin') {
    return null;
  }

  const handleAdd = async () => {
    const name = newUsername.trim();
    if (!name) {
      return;
    }

    await addEntry.mutateAsync(name);
    setNewUsername('');
  };

  const handleRemove = async (username: string) => {
    if (!window.confirm(t('admin.confirmRemove', { username }))) {
      return;
    }

    await removeEntry.mutateAsync(username);
  };

  return (
    <main className="h-full overflow-y-auto bg-[var(--bg-body)] pb-[env(safe-area-inset-bottom)] text-[var(--text-body)]">
      <div className="mx-auto flex max-w-4xl flex-col gap-6 px-4 py-8 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between gap-4">
          <Link href="/" className="text-sm text-[var(--text-muted)] transition-colors hover:text-[var(--text-body)]">
            ← {t('admin.back')}
          </Link>
          <p className="text-sm text-[var(--text-muted)]">{t('admin.currentAdmin', { username: user.github_username })}</p>
        </div>

        <section className="rounded-2xl border border-[var(--border-strong)] bg-[var(--bg-input)]/70 p-5 shadow-sm shadow-[var(--border-default)]/40 backdrop-blur-sm sm:p-6">
          <h1 className="font-serif text-3xl font-semibold tracking-tight">{t('admin.title')}</h1>
          <p className="mt-2 text-sm leading-6 text-[var(--text-muted)]">
            {t('admin.description')}
          </p>

          <form
            className="mt-5 flex flex-col gap-3 sm:flex-row"
            onSubmit={(e) => {
              e.preventDefault();
              void handleAdd();
            }}
          >
            <input
              type="text"
              value={newUsername}
              onChange={(e) => setNewUsername(e.target.value)}
              placeholder={t('admin.usernamePlaceholder')}
              className="min-w-0 flex-1 rounded-xl border border-[var(--border-strong)] bg-[var(--bg-body)] px-4 py-2.5 text-sm outline-none transition focus:border-[var(--border-accent)]"
            />
            <button
              type="submit"
              disabled={addEntry.isPending}
              className="rounded-xl bg-[var(--accent)] px-5 py-2.5 text-sm font-medium text-[var(--text-body)] transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {addEntry.isPending ? t('admin.adding') : t('admin.add')}
            </button>
          </form>
        </section>

        <section className="rounded-2xl border border-[var(--border-strong)] bg-[var(--bg-input)]/70 p-5 shadow-sm shadow-[var(--border-default)]/40 backdrop-blur-sm sm:p-6">
          <div className="mb-4 flex items-center justify-between gap-3">
            <h2 className="text-lg font-semibold">{t('admin.allowedUsers')}</h2>
            <span className="text-sm text-[var(--text-muted)]">{t('admin.userCount', { count: entries?.length ?? 0 })}</span>
          </div>

          {isLoading ? (
            <p className="py-8 text-center text-sm text-[var(--text-muted)]">{t('admin.loading')}</p>
          ) : !entries?.length ? (
            <p className="py-8 text-center text-sm text-[var(--text-muted)]">{t('admin.empty')}</p>
          ) : (
            <div className="overflow-hidden rounded-xl border border-[var(--border-strong)]">
              <table className="w-full border-collapse text-sm">
                <thead className="bg-[var(--bg-body)] text-left text-[var(--text-muted)]">
                  <tr>
                    <th className="px-4 py-3 font-medium">{t('admin.username')}</th>
                    <th className="px-4 py-3 font-medium">{t('admin.addedAt')}</th>
                    <th className="px-4 py-3 font-medium text-right">{t('admin.actions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-strong)] bg-[var(--bg-input)]">
                  {entries.map((entry) => (
                    <tr key={entry.github_username}>
                      <td className="px-4 py-3 font-medium">{entry.github_username}</td>
                      <td className="px-4 py-3 text-[var(--text-muted)]">{formatAddedAt(entry.added_at)}</td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => void handleRemove(entry.github_username)}
                          disabled={removeEntry.isPending}
                          className="inline-flex min-h-11 items-center justify-center rounded-lg border border-[var(--border-strong)] px-3 py-1.5 text-xs font-medium text-[var(--text-body)] transition hover:border-[var(--border-accent)] hover:text-[var(--text-accent)] disabled:cursor-not-allowed disabled:opacity-60 md:min-h-0"
                        >
                          {removeEntry.isPending ? t('admin.processing') : t('admin.remove')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
