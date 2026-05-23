'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api-client';
import { useI18n } from '@/lib/i18n';
import { useAuthStore } from '@/stores/useAuthStore';

interface GuestSettings {
  enabled: boolean;
}

interface AISettings {
  endpoint: string;
  model: string;
  api_key_set: boolean;
  api_key_hint?: string;
}

interface FeverKeyResponse {
  api_key: string;
  fever_url: string;
  username: string;
}

export default function SettingsPage() {
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const isAdmin = useAuthStore((state) => state.user?.role === 'admin');
  const isGuest = useAuthStore((state) => state.user?.role === 'guest');
  const [endpoint, setEndpoint] = useState('');
  const [model, setModel] = useState('');
  const [apiKey, setAPIKey] = useState('');
  const [message, setMessage] = useState('');

  // Fever API state
  const [feverPassword, setFeverPassword] = useState('');
  const [feverResult, setFeverResult] = useState<FeverKeyResponse | null>(null);
  const [feverMessage, setFeverMessage] = useState('');
  const [feverCopied, setFeverCopied] = useState<'key' | 'url' | null>(null);

  const { data: aiSettings, error: aiSettingsError } = useQuery({
    queryKey: ['ai-settings'],
    queryFn: () => apiFetch<AISettings>('/api/ai/settings'),
  });

  useEffect(() => {
    if (!aiSettings) return;
    let cancelled = false;
    queueMicrotask(() => {
      if (cancelled) return;
      setEndpoint(aiSettings.endpoint);
      setModel(aiSettings.model);
    });
    return () => {
      cancelled = true;
    };
  }, [aiSettings]);

  const saveAISettings = useMutation({
    mutationFn: () =>
      apiFetch<AISettings>('/api/ai/settings', {
        method: 'PATCH',
        body: JSON.stringify({
          endpoint,
          model,
          api_key: apiKey,
        }),
      }),
    onSuccess: (settings) => {
      queryClient.setQueryData(['ai-settings'], settings);
      setEndpoint(settings.endpoint);
      setModel(settings.model);
      setAPIKey('');
      setMessage(t('settings.aiSaved'));
    },
    onError: (err: Error) => setMessage(`${t('settings.aiSaveError')} ${err.message}`),
  });

  const generateFeverKey = useMutation({
    mutationFn: () =>
      apiFetch<FeverKeyResponse>('/api/users/me/fever', {
        method: 'POST',
        body: JSON.stringify({ password: feverPassword }),
      }),
    onSuccess: (result) => {
      setFeverResult(result);
      setFeverPassword('');
      setFeverMessage(t('settings.feverGenerated'));
    },
    onError: () => setFeverMessage(t('settings.feverError')),
  });

  const { data: guestSettings } = useQuery({
    queryKey: ['guest-settings'],
    queryFn: () => apiFetch<GuestSettings>('/api/settings/guest'),
    enabled: isAdmin,
  });

  const toggleGuestMode = useMutation({
    mutationFn: (enabled: boolean) =>
      apiFetch('/api/settings/guest', {
        method: 'PATCH',
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['guest-settings'] }),
    onError: () => setMessage(t('settings.guestModeSaveError')),
  });

  const copyToClipboard = async (text: string, kind: 'key' | 'url') => {
    await navigator.clipboard.writeText(text);
    setFeverCopied(kind);
    setTimeout(() => setFeverCopied(null), 2000);
  };

  return (
    <main className="h-full overflow-y-auto bg-[var(--bg-body)] pb-[env(safe-area-inset-bottom)] text-[var(--text-body)]">
      <div className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="mb-6 flex items-center justify-between gap-4">
          <Link href="/" className="font-[system-ui] text-sm text-[var(--text-muted)] transition-colors hover:text-[var(--text-body)]">
            ← {t('settings.backHome')}
          </Link>
          <div className="font-[system-ui] text-sm text-[var(--text-muted)]">
            {t('settings.breadcrumb')}
          </div>
        </div>

        <header className="mb-8 space-y-3">
          <h1 className="font-serif text-4xl font-semibold tracking-tight text-[var(--text-body)]">{t('settings.title')}</h1>
          <p className="max-w-2xl font-[system-ui] text-sm leading-6 text-[var(--text-muted)]">
            {t('settings.description')}
          </p>
        </header>

        <section className="rounded-[8px] border border-[var(--border-light)] bg-[var(--bg)] p-5 shadow-[0_18px_40px_rgba(65,52,35,0.06)]">
          <div className="mb-5">
            <h2 className="font-serif text-2xl font-semibold text-[var(--text-body)]">{t('settings.aiTitle')}</h2>
            <p className="mt-2 font-[system-ui] text-sm leading-6 text-[var(--text-muted)]">{t('settings.aiDescription')}</p>
          </div>

          {aiSettingsError ? (
            <p className="mb-4 rounded-[7px] border border-[var(--bg-highlight-error)] bg-[var(--bg-highlight-error)]/30 px-3 py-2 font-[system-ui] text-xs text-[var(--text-body)]">
              {(aiSettingsError as Error).message}
            </p>
          ) : null}

          <div className="space-y-4 font-[system-ui]">
            <label className="block text-sm font-medium text-[var(--text-body)]">
              {t('settings.aiEndpoint')}
              <input
                className={`mt-2 w-full rounded-[7px] border border-[var(--border-light)] px-3 py-2 text-sm outline-none transition-colors focus:border-[var(--accent)] ${isAdmin ? 'bg-[var(--bg-body)]' : 'cursor-not-allowed bg-[var(--bg-panel)] text-[var(--text-muted)]'}`}
                value={endpoint}
                readOnly={!isAdmin}
                onChange={(event) => setEndpoint(event.target.value)}
              />
            </label>

            <label className="block text-sm font-medium text-[var(--text-body)]">
              {t('settings.aiModel')}
              <input
                list={isAdmin ? 'ai-model-options' : undefined}
                className={`mt-2 w-full rounded-[7px] border border-[var(--border-light)] px-3 py-2 text-sm outline-none transition-colors focus:border-[var(--accent)] ${isAdmin ? 'bg-[var(--bg-body)]' : 'cursor-not-allowed bg-[var(--bg-panel)] text-[var(--text-muted)]'}`}
                value={model}
                readOnly={!isAdmin}
                onChange={(event) => setModel(event.target.value)}
              />
              {isAdmin ? (
                <datalist id="ai-model-options">
                  <option value="qwen-turbo" />
                  <option value="deepseek-chat" />
                  <option value="gpt-4o-mini" />
                  <option value="gpt-4.1-mini" />
                </datalist>
              ) : null}
            </label>

            <label className="block text-sm font-medium text-[var(--text-body)]">
              {t('settings.aiApiKey')}
              <input
                type="password"
                autoComplete="new-password"
                placeholder={isAdmin ? t('settings.aiApiKeyPlaceholder') : ''}
                className={`mt-2 w-full rounded-[7px] border border-[var(--border-light)] px-3 py-2 text-sm outline-none transition-colors focus:border-[var(--accent)] ${isAdmin ? 'bg-[var(--bg-body)]' : 'cursor-not-allowed bg-[var(--bg-panel)] text-[var(--text-muted)]'}`}
                value={apiKey}
                readOnly={!isAdmin}
                onChange={(event) => setAPIKey(event.target.value)}
              />
            </label>

            <p className="text-xs text-[var(--text-muted)]">
              {aiSettings?.api_key_set
                ? t('settings.aiCurrentKey', { hint: aiSettings.api_key_hint || '***' })
                : t('settings.aiNoKey')}
            </p>
            {!isAdmin ? <p className="text-xs text-[var(--text-muted)]">{t('settings.aiAdminOnly')}</p> : null}

            <div className="flex items-center gap-3">
              {isAdmin ? (
                <button
                  type="button"
                  onClick={() => { setMessage(''); saveAISettings.mutate(); }}
                  disabled={saveAISettings.isPending}
                  className="rounded-[7px] bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-white transition-opacity disabled:opacity-60"
                >
                  {saveAISettings.isPending ? t('settings.saving') : t('settings.aiSave')}
                </button>
              ) : null}
              {message ? <span className="text-sm text-[var(--text-muted)]">{message}</span> : null}
            </div>
          </div>
        </section>

        {isAdmin && (
          <section className="mt-6 rounded-[8px] border border-[var(--border-light)] bg-[var(--bg)] p-5 shadow-[0_18px_40px_rgba(65,52,35,0.06)]">
            <div className="mb-5">
              <h2 className="font-serif text-2xl font-semibold text-[var(--text-body)]">{t('settings.guestMode')}</h2>
              <p className="mt-2 font-[system-ui] text-sm leading-6 text-[var(--text-muted)]">{t('settings.guestModeDesc')}</p>
            </div>
            <div className="flex items-center gap-3 font-[system-ui]">
              <button
                type="button"
                role="switch"
                aria-checked={guestSettings?.enabled ?? false}
                onClick={() => toggleGuestMode.mutate(!(guestSettings?.enabled ?? false))}
                disabled={toggleGuestMode.isPending || guestSettings === undefined}
                className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none disabled:cursor-not-allowed disabled:opacity-60 ${(guestSettings?.enabled ?? false) ? 'bg-[var(--accent)]' : 'bg-[var(--border-light)]'}`}
              >
                <span
                  className={`inline-block h-4 w-4 rounded-full bg-white shadow ring-0 transition-transform duration-200 ease-in-out ${(guestSettings?.enabled ?? false) ? 'translate-x-5' : 'translate-x-0'}`}
                />
              </button>
              <span className="text-sm text-[var(--text-muted)]">
                {(guestSettings?.enabled ?? false) ? t('settings.guestModeEnabled') : t('settings.guestModeDisabled')}
              </span>
            </div>
          </section>
        )}

        {!isGuest && <section className="mt-6 rounded-[8px] border border-[var(--border-light)] bg-[var(--bg)] p-5 shadow-[0_18px_40px_rgba(65,52,35,0.06)]">
          <div className="mb-5">
            <h2 className="font-serif text-2xl font-semibold text-[var(--text-body)]">{t('settings.feverTitle')}</h2>
            <p className="mt-2 font-[system-ui] text-sm leading-6 text-[var(--text-muted)]">{t('settings.feverDescription')}</p>
          </div>

          <div className="space-y-4 font-[system-ui]">
            {!feverResult ? (
              <>
                <label className="block text-sm font-medium text-[var(--text-body)]">
                  {t('settings.feverPassword')}
                  <input
                    type="password"
                    autoComplete="new-password"
                    placeholder={t('settings.feverPasswordPlaceholder')}
                    className="mt-2 w-full rounded-[7px] border border-[var(--border-light)] bg-[var(--bg-body)] px-3 py-2 text-sm outline-none transition-colors focus:border-[var(--accent)]"
                    value={feverPassword}
                    onChange={(event) => setFeverPassword(event.target.value)}
                  />
                </label>

                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    onClick={() => generateFeverKey.mutate()}
                    disabled={generateFeverKey.isPending || feverPassword.length < 6}
                    className="rounded-[7px] bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-white transition-opacity disabled:opacity-60"
                  >
                    {generateFeverKey.isPending ? t('settings.saving') : t('settings.feverGenerate')}
                  </button>
                  {feverMessage && !feverResult ? <span className="text-sm text-[var(--text-muted)]">{feverMessage}</span> : null}
                </div>
              </>
            ) : (
              <div className="space-y-3">
                <div className="rounded-[7px] border border-[var(--border-light)] bg-[var(--bg-body)] p-4">
                  <div className="mb-3 text-xs font-medium uppercase tracking-wider text-[var(--text-muted)]">{t('settings.feverUrlLabel')}</div>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 break-all text-sm text-[var(--text-body)]">
                      {`${typeof window !== 'undefined' ? window.location.origin : ''}/fever/`}
                    </code>
                    <button
                      type="button"
                      onClick={() => copyToClipboard(`${window.location.origin}/fever/`, 'url')}
                      className="shrink-0 rounded-[5px] border border-[var(--border-light)] px-2 py-1 text-xs text-[var(--text-muted)] transition-colors hover:text-[var(--text-body)]"
                    >
                      {feverCopied === 'url' ? t('settings.feverCopied') : t('settings.feverCopy')}
                    </button>
                  </div>
                </div>

                <div className="rounded-[7px] border border-[var(--border-light)] bg-[var(--bg-body)] p-4">
                  <div className="mb-3 text-xs font-medium uppercase tracking-wider text-[var(--text-muted)]">{t('settings.feverApiKeyLabel')}</div>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 break-all font-mono text-sm text-[var(--text-body)]">{feverResult.api_key}</code>
                    <button
                      type="button"
                      onClick={() => copyToClipboard(feverResult.api_key, 'key')}
                      className="shrink-0 rounded-[5px] border border-[var(--border-light)] px-2 py-1 text-xs text-[var(--text-muted)] transition-colors hover:text-[var(--text-body)]"
                    >
                      {feverCopied === 'key' ? t('settings.feverCopied') : t('settings.feverCopy')}
                    </button>
                  </div>
                </div>

                <p className="text-xs text-[var(--text-muted)]">{t('settings.feverKeyNote')}</p>
                <p className="text-xs text-[var(--text-muted)]">{t('settings.feverInstructions')}</p>

                <button
                  type="button"
                  onClick={() => {
                    setFeverResult(null);
                    setFeverMessage('');
                  }}
                  className="text-sm text-[var(--accent)] hover:underline"
                >
                  {t('settings.feverRegenerate')}
                </button>
              </div>
            )}
          </div>
        </section>}
      </div>
    </main>
  );
}
