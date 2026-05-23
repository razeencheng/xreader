'use client';

import { useEffect, useRef } from 'react';
import { usePathname } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { X } from 'lucide-react';
import {
  getSourceImportCompleted,
  getSourceImportProgress,
  isSourceImportLookupExpired,
  useSourceImportJob,
} from '@/lib/queries/sources';
import { useI18n } from '@/lib/i18n';
import { useUIStore } from '@/stores/useUIStore';

export function SourceImportStatus() {
  const pathname = usePathname();
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const sourceImportJob = useUIStore((state) => state.sourceImportJob);
  const clearSourceImport = useUIStore((state) => state.clearSourceImport);
  const handledJobIdRef = useRef<string | null>(null);
  const importJob = useSourceImportJob(sourceImportJob?.id ?? null);
  const status = importJob.data;
  const lookupExpired = isSourceImportLookupExpired(importJob.error);
  const isFailed = !lookupExpired && (importJob.isError || status?.status === 'failed');
  const isDone = status?.status === 'done';

  useEffect(() => {
    if (sourceImportJob && lookupExpired) {
      clearSourceImport();
    }
  }, [clearSourceImport, lookupExpired, sourceImportJob]);

  useEffect(() => {
    if (!sourceImportJob || handledJobIdRef.current === sourceImportJob.id) return;
    if (!isDone && !isFailed) return;

    handledJobIdRef.current = sourceImportJob.id;
    void queryClient.invalidateQueries({ queryKey: ['sources'] });
  }, [isDone, isFailed, queryClient, sourceImportJob]);

  if (!sourceImportJob || pathname === '/sources' || lookupExpired) {
    return null;
  }

  const total = status?.total && status.total > 0 ? status.total : 100;
  const completed = status?.total ? Math.min(total, getSourceImportCompleted(status)) : Math.round(getSourceImportProgress(status));
  const progress = getSourceImportProgress(status);
  const state = isFailed ? t('sources.importFailed') : isDone ? t('sources.importDoneState') : t('sources.importPolling');

  return (
    <div className="pointer-events-none fixed inset-x-3 bottom-3 z-50 flex justify-center md:bottom-5 lg:left-[68px]">
      <div
        role="status"
        aria-live="polite"
        className="pointer-events-auto w-full max-w-[520px] rounded-[8px] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-3 text-[var(--text-body)] shadow-[0_18px_55px_rgba(47,39,26,0.16)]"
      >
        <div className="flex min-w-0 items-start gap-3">
          <div className="min-w-0 flex-1">
            <div className="truncate text-[13px] font-semibold text-[var(--text-1)]">
              {t('sources.importingFile', { filename: sourceImportJob.fileName })}
            </div>
            <div className="mt-1 text-[12px] text-[var(--text-muted)]">
              {t('sources.importProgressFeeds', {
                done: completed,
                total,
                state,
              })}
            </div>
          </div>
          {isDone || isFailed ? (
            <button
              type="button"
              aria-label={t('sources.dismiss')}
              title={t('sources.dismiss')}
              onClick={clearSourceImport}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-1)]"
            >
              <X size={15} strokeWidth={1.75} />
            </button>
          ) : null}
        </div>
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-[var(--border-light)]">
          <div
            className={`h-full rounded-full ${isFailed ? 'bg-[var(--text-error)]' : 'bg-[var(--accent)]'}`}
            style={{ width: `${progress || 8}%` }}
          />
        </div>
      </div>
    </div>
  );
}
