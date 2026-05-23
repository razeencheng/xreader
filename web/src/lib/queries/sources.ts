import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError, apiFetch } from '@/lib/api-client';
import type { Source } from '@/lib/types';

export interface SourceImportJobStatus {
  status: 'pending' | 'running' | 'done' | 'failed';
  progress?: number;
  total?: number;
  succeeded?: number;
  failed?: number;
  skipped?: number;
  error?: string;
}

function clampPercent(value: number) {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

export function getSourceImportCompleted(status: SourceImportJobStatus | null | undefined) {
  if (!status) return 0;
  return Math.max(0, (status.succeeded ?? 0) + (status.failed ?? 0) + (status.skipped ?? 0));
}

export function getSourceImportProgress(status: SourceImportJobStatus | null | undefined) {
  if (!status) return 0;
  if (typeof status.progress === 'number') {
    return clampPercent(status.progress <= 1 ? status.progress * 100 : status.progress);
  }
  if (status.total && status.total > 0) {
    return clampPercent((getSourceImportCompleted(status) / status.total) * 100);
  }
  return status.status === 'done' ? 100 : 0;
}

export function isSourceImportLookupExpired(error: unknown) {
  return error instanceof ApiError && (error.status === 401 || error.status === 404);
}

export function useSources() {
  return useQuery<Source[]>({
    queryKey: ['sources'],
    queryFn: () => apiFetch<Source[]>('/api/sources'),
  });
}

export function useCreateSource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<Source>('/api/sources', {
        method: 'POST',
        body: JSON.stringify({ url }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sources'] }),
  });
}

export function useRenameSource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, title }: { id: number; title: string }) =>
      apiFetch<void>(`/api/sources/${id}`, {
        method: 'PUT',
        body: JSON.stringify({ title }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sources'] }),
  });
}

export function useRefreshSource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/sources/${id}/refresh`, {
        method: 'POST',
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sources'] }),
  });
}

export function useDeleteSource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/sources/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sources'] }),
  });
}

export function useSourceImportJob(jobId: string | null) {
  return useQuery<SourceImportJobStatus>({
    queryKey: ['sources', 'jobs', jobId],
    queryFn: () => apiFetch<SourceImportJobStatus>(`/api/sources/jobs/${jobId}`, { redirectOnUnauthorized: false }),
    enabled: Boolean(jobId),
    retry: false,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === 'pending' || status === 'running' ? 1000 : false;
    },
  });
}
