'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import {
  Check,
  ChevronLeft,
  Download,
  ExternalLink,
  Grid2X2,
  List,
  Plus,
  RefreshCw,
  Search,
  Settings2,
  Trash2,
  Upload,
  X,
} from 'lucide-react';
import { ApiError, apiFetch } from '@/lib/api-client';
import { useI18n } from '@/lib/i18n';
import { useIsGuest } from '@/stores/useAuthStore';
import {
  getSourceImportCompleted,
  getSourceImportProgress,
  isSourceImportLookupExpired,
  useCreateSource,
  useDeleteSource,
  useRefreshSource,
  useSourceImportJob,
  useSources,
} from '@/lib/queries/sources';
import { useUIStore } from '@/stores/useUIStore';
import type { Source } from '@/lib/types';
import styles from './SourcesPage.module.css';

type SourceStatus = 'healthy' | 'stale' | 'error';
type StatusFilter = 'all' | SourceStatus;
type SortMode = 'recent' | 'title' | 'items';
type Density = 'compact' | 'comfortable';
type ViewMode = 'list' | 'cards';
type Tone = 'quiet' | 'editorial';
type DiscoveryPhase = 'idle' | 'detecting' | 'found' | 'error';

type Discovery = {
  title: string;
  site: string;
  url: string;
  submitUrl: string;
  items: number;
};

type SourceViewModel = Source & {
  site: string;
  status: SourceStatus;
  lastCheckedMs: number;
  itemCount: number;
  errorReason: string;
};

const STARTER_PACKS = [
  { title: 'simonwillison.net', url: 'https://simonwillison.net/atom/everything/' },
  { title: 'Overreacted by Dan Abramov', url: 'https://overreacted.io/rss.xml' },
  { title: 'Joel on Software', url: 'https://www.joelonsoftware.com/feed/' },
] as const;

const STATUS_FILTERS: Array<{ id: StatusFilter; labelKey: string }> = [
  { id: 'all', labelKey: 'sources.filterAll' },
  { id: 'healthy', labelKey: 'sources.filterHealthy' },
  { id: 'stale', labelKey: 'sources.filterStale' },
  { id: 'error', labelKey: 'sources.filterErrors' },
];

function getHostFromUrl(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';

  try {
    return new URL(trimmed.includes('://') ? trimmed : `https://${trimmed}`).hostname.replace(/^www\./, '');
  } catch {
    return trimmed.replace(/^https?:\/\//, '').split('/')[0].replace(/^www\./, '');
  }
}

function normalizeComparableUrl(value: string) {
  return value.trim().toLowerCase().replace(/^https?:\/\//, '').replace(/^www\./, '').replace(/\/$/, '');
}

function titleFromSite(site: string) {
  const firstPart = site.split('.')[0] || site;
  return firstPart
    .replace(/[-_]/g, ' ')
    .replace(/\b\w/g, (character) => character.toUpperCase());
}

function faviconUrl(site: string) {
  return site ? `https://www.google.com/s2/favicons?domain=${encodeURIComponent(site)}&sz=64` : '';
}

function isDomainLike(value: string) {
  const host = getHostFromUrl(value);
  return /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/i.test(host);
}

function buildDiscovery(value: string): Discovery {
  const submitUrl = value.trim();
  const site = getHostFromUrl(submitUrl);
  const hasPath = submitUrl.replace(/^https?:\/\//, '').includes('/');
  const url = hasPath
    ? submitUrl.includes('://')
      ? submitUrl
      : `https://${submitUrl}`
    : `https://${site}/feed.xml`;
  const seed = Array.from(site).reduce((sum, character) => sum + character.charCodeAt(0), 0);

  return {
    title: titleFromSite(site) || 'Untitled feed',
    site,
    url,
    submitUrl,
    items: 20 + (seed % 800),
  };
}

function statusForSource(source: Source): SourceStatus {
  const health = (source.health || '').toLowerCase();
  if (health.includes('error') || health.includes('fail') || source.consecutive_fails >= 6) return 'error';
  if (health.includes('warn') || health.includes('degraded')) return 'stale';

  const lastChecked = Date.parse(source.last_fetched_at || source.last_success_at || '');
  if (Number.isFinite(lastChecked) && Date.now() - lastChecked > 1000 * 60 * 60 * 24 * 7) {
    return 'stale';
  }

  return 'healthy';
}

function formatRelativeTime(
  timestamp: number,
  t: (key: string, params?: Record<string, string | number>) => string,
) {
  if (!timestamp) return t('sources.never');
  const diffMinutes = Math.floor((Date.now() - timestamp) / 60000);
  if (diffMinutes < 1) return t('sources.justNow');
  if (diffMinutes < 60) return t('sources.minutesAgo', { count: diffMinutes });

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return t('sources.hoursAgo', { count: diffHours });

  return t('sources.daysAgo', { count: Math.floor(diffHours / 24) });
}

function sourceToViewModel(source: Source, refreshedAt: Map<number, number>): SourceViewModel {
  const site = getHostFromUrl(source.url);
  const refreshedTimestamp = refreshedAt.get(source.id);
  const parsedLastChecked = Date.parse(source.last_fetched_at || source.last_success_at || '');
  const lastCheckedMs = refreshedTimestamp ?? (Number.isFinite(parsedLastChecked) ? parsedLastChecked : 0);

  return {
    ...source,
    site,
    status: statusForSource(source),
    lastCheckedMs,
    itemCount: source.unread_count ?? 0,
    errorReason: source.consecutive_fails > 0 ? `${source.consecutive_fails} consecutive failures` : 'Connection timed out',
  };
}

function StatusPill({
  status,
  t,
}: {
  status: SourceStatus;
  t: (key: string, params?: Record<string, string | number>) => string;
}) {
  const labelKey = status === 'healthy' ? 'sources.statusHealthy' : status === 'stale' ? 'sources.statusStale' : 'sources.statusError';

  return (
    <span className={`${styles.statusPill} ${styles[status]}`}>
      <span aria-hidden="true">●</span>
      <span>{t(labelKey)}</span>
    </span>
  );
}

function SourceFavicon({ source }: { source: SourceViewModel }) {
  return (
    <span className={styles.favicon} aria-hidden="true">
      <span
        className={styles.faviconImage}
        style={{ backgroundImage: `url("${source.icon_url || faviconUrl(source.site)}")` }}
      />
    </span>
  );
}

function SourceRow({
  source,
  density,
  refreshing,
  onRefresh,
  onDelete,
  isGuest,
  t,
}: {
  source: SourceViewModel;
  density: Density;
  refreshing: boolean;
  onRefresh: (source: SourceViewModel) => void;
  onDelete: (source: SourceViewModel) => void;
  isGuest: boolean;
  t: (key: string, params?: Record<string, string | number>) => string;
}) {
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const compact = density === 'compact';

  return (
    <div className={`${styles.row} ${compact ? styles.compact : ''}`} data-source-row>
      <div className={styles.rowMain}>
        <SourceFavicon source={source} />
        <div className={styles.rowText}>
          <div className={styles.rowTitle}>{source.title}</div>
          <div className={styles.rowUrl} title={source.url}>{source.url}</div>
        </div>
      </div>

      <div className={styles.rowMeta}>
        <StatusPill status={source.status} t={t} />
        <div className={styles.rowTime}>{formatRelativeTime(source.lastCheckedMs, t)}</div>
        <div className={styles.rowItems}>{t('sources.itemCount', { count: source.itemCount })}</div>
      </div>

      <div className={styles.rowActions}>
        <button
          type="button"
          className={styles.iconButton}
          aria-label={t('sources.openSource')}
          onClick={() => window.open(source.url, '_blank', 'noopener,noreferrer')}
        >
          <ExternalLink size={14} strokeWidth={1.5} />
        </button>
        <button
          type="button"
          className={`${styles.iconButton} ${refreshing ? styles.spinningIcon : ''}`}
          aria-label={t('sources.refreshSource')}
          onClick={() => onRefresh(source)}
          disabled={refreshing}
        >
          <RefreshCw size={14} strokeWidth={1.5} />
        </button>
        {!isGuest && (confirmingDelete ? (
          <span className={styles.confirmPop}>
            <button
              type="button"
              className={`${styles.iconButton} ${styles.dangerButton}`}
              aria-label={t('sources.confirmDelete')}
              onClick={() => {
                onDelete(source);
                setConfirmingDelete(false);
              }}
            >
              <Check size={14} strokeWidth={1.5} />
            </button>
            <button
              type="button"
              className={styles.iconButton}
              aria-label={t('sources.cancelDelete')}
              onClick={() => setConfirmingDelete(false)}
            >
              <X size={14} strokeWidth={1.5} />
            </button>
          </span>
        ) : (
          <button
            type="button"
            className={styles.iconButton}
            aria-label={t('sources.unsubscribe')}
            onClick={() => setConfirmingDelete(true)}
          >
            <Trash2 size={14} strokeWidth={1.5} />
          </button>
        ))}
      </div>

      {source.status === 'error' ? (
        <div className={styles.rowError}>
          <span>{t('sources.lastFetchFailed', { reason: source.errorReason })}</span>
          <button type="button" className={styles.buttonText} onClick={() => onRefresh(source)}>
            {t('sources.retry')}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function SourceCard({
  source,
  refreshing,
  onRefresh,
  onDelete,
  isGuest,
  t,
}: {
  source: SourceViewModel;
  refreshing: boolean;
  onRefresh: (source: SourceViewModel) => void;
  onDelete: (source: SourceViewModel) => void;
  isGuest: boolean;
  t: (key: string, params?: Record<string, string | number>) => string;
}) {
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  return (
    <article className={styles.card}>
      <div className={styles.cardHead}>
        <SourceFavicon source={source} />
        <StatusPill status={source.status} t={t} />
      </div>
      <div className={styles.cardTitle}>{source.title}</div>
      <div className={styles.cardUrl}>{source.site}</div>
      <div className={styles.cardMeta}>
        <span>{t('sources.itemCount', { count: source.itemCount })}</span>
        <span>·</span>
        <span>{formatRelativeTime(source.lastCheckedMs, t)}</span>
      </div>
      <div className={styles.cardActions}>
        <button
          type="button"
          className={`${styles.iconButton} ${refreshing ? styles.spinningIcon : ''}`}
          aria-label={t('sources.refreshSource')}
          onClick={() => onRefresh(source)}
          disabled={refreshing}
        >
          <RefreshCw size={14} strokeWidth={1.5} />
        </button>
        <button
          type="button"
          className={styles.iconButton}
          aria-label={t('sources.openSource')}
          onClick={() => window.open(source.url, '_blank', 'noopener,noreferrer')}
        >
          <ExternalLink size={14} strokeWidth={1.5} />
        </button>
        {!isGuest && (confirmingDelete ? (
          <>
            <button
              type="button"
              className={`${styles.iconButton} ${styles.dangerButton}`}
              aria-label={t('sources.confirmDelete')}
              onClick={() => {
                onDelete(source);
                setConfirmingDelete(false);
              }}
            >
              <Check size={14} strokeWidth={1.5} />
            </button>
            <button
              type="button"
              className={styles.iconButton}
              aria-label={t('sources.cancelDelete')}
              onClick={() => setConfirmingDelete(false)}
            >
              <X size={14} strokeWidth={1.5} />
            </button>
          </>
        ) : (
          <button
            type="button"
            className={styles.iconButton}
            aria-label={t('sources.unsubscribe')}
            onClick={() => setConfirmingDelete(true)}
          >
            <Trash2 size={14} strokeWidth={1.5} />
          </button>
        ))}
      </div>
    </article>
  );
}

function SegmentButton<T extends string>({
  value,
  current,
  onSelect,
  children,
}: {
  value: T;
  current: T;
  onSelect: (value: T) => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      className={value === current ? styles.segmentActive : undefined}
      onClick={() => onSelect(value)}
    >
      {children}
    </button>
  );
}

export function SourcesPage() {
  const { t } = useI18n();
  const isGuest = useIsGuest();
  const { data: sources, isLoading } = useSources();
  const createSource = useCreateSource();
  const deleteSource = useDeleteSource();
  const refreshSource = useRefreshSource();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const handledImportJobId = useRef<string | null>(null);
  const discoveryTimerRef = useRef<number | null>(null);

  const [sourceUrl, setSourceUrl] = useState('');
  const [discoveryPhase, setDiscoveryPhase] = useState<DiscoveryPhase>('idle');
  const [discovery, setDiscovery] = useState<Discovery | null>(null);
  const [discoveryError, setDiscoveryError] = useState('');
  const [deletedSourceIds, setDeletedSourceIds] = useState<Set<number>>(new Set());
  const [refreshedAt, setRefreshedAt] = useState<Map<number, number>>(new Map());
  const [refreshingIds, setRefreshingIds] = useState<Set<number>>(new Set());
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [filterQuery, setFilterQuery] = useState('');
  const [sortMode, setSortMode] = useState<SortMode>('recent');
  const [density, setDensity] = useState<Density>('comfortable');
  const [viewMode, setViewMode] = useState<ViewMode>('list');
  const [tone, setTone] = useState<Tone>('quiet');
  const [showFavicons, setShowFavicons] = useState(true);
  const [isTweaksOpen, setIsTweaksOpen] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const sourceImportJob = useUIStore((state) => state.sourceImportJob);
  const startSourceImport = useUIStore((state) => state.startSourceImport);
  const clearSourceImport = useUIStore((state) => state.clearSourceImport);

  const importJob = useSourceImportJob(sourceImportJob?.id ?? null);

  const visibleSources = useMemo(() => {
    if (!sources) return [];
    return sources.filter((source) => !deletedSourceIds.has(source.id));
  }, [deletedSourceIds, sources]);

  const sourceRows = useMemo(
    () => visibleSources.map((source) => sourceToViewModel(source, refreshedAt)),
    [refreshedAt, visibleSources],
  );

  const counts = useMemo(() => ({
    all: sourceRows.length,
    healthy: sourceRows.filter((source) => source.status === 'healthy').length,
    stale: sourceRows.filter((source) => source.status === 'stale').length,
    error: sourceRows.filter((source) => source.status === 'error').length,
  }), [sourceRows]);
  const attentionCount = counts.stale + counts.error;

  const filteredSources = useMemo(() => {
    const query = filterQuery.trim().toLowerCase();
    let list = sourceRows;

    if (statusFilter !== 'all') {
      list = list.filter((source) => source.status === statusFilter);
    }

    if (query) {
      list = list.filter((source) =>
        source.title.toLowerCase().includes(query) ||
        source.url.toLowerCase().includes(query) ||
        source.site.toLowerCase().includes(query));
    }

    return [...list].sort((left, right) => {
      if (sortMode === 'title') return left.title.localeCompare(right.title);
      if (sortMode === 'items') return right.itemCount - left.itemCount;
      return right.lastCheckedMs - left.lastCheckedMs;
    });
  }, [filterQuery, sortMode, sourceRows, statusFilter]);

  const importProgress = getSourceImportProgress(importJob.data);
  const importTotal = importJob.data?.total && importJob.data.total > 0 ? importJob.data.total : 100;
  const importCompleted = importJob.data?.total ? Math.min(importTotal, getSourceImportCompleted(importJob.data)) : Math.round(importProgress);
  const importDone = importJob.data?.status === 'done';
  const importLookupExpired = isSourceImportLookupExpired(importJob.error);
  const importFailed = !importLookupExpired && (importJob.isError || importJob.data?.status === 'failed');
  const importRunning = Boolean(sourceImportJob && !importLookupExpired);
  const latestSync = sourceRows.reduce((latest, source) => Math.max(latest, source.lastCheckedMs), 0);

  useEffect(() => () => {
    if (discoveryTimerRef.current != null) {
      window.clearTimeout(discoveryTimerRef.current);
    }
  }, []);

  useEffect(() => {
    if (!sourceImportJob?.id) return;
    if (importLookupExpired) {
      clearSourceImport();
      return;
    }
    if (!importJob.data) return;
    if (handledImportJobId.current === sourceImportJob.id) return;

    if (importJob.data.status === 'done') {
      handledImportJobId.current = sourceImportJob.id;
      showToast(t('sources.importCompleteMessage'));
    }

    if (importJob.data.status === 'failed') {
      handledImportJobId.current = sourceImportJob.id;
      showToast(t('sources.importFailedMessage'));
    }
  }, [clearSourceImport, importJob.data, importLookupExpired, sourceImportJob?.id, t]);

  function showToast(message: string) {
    setToast(message);
    window.setTimeout(() => setToast(null), 2400);
  }

  function resetDiscovery() {
    if (discoveryTimerRef.current != null) {
      window.clearTimeout(discoveryTimerRef.current);
      discoveryTimerRef.current = null;
    }
    setSourceUrl('');
    setDiscovery(null);
    setDiscoveryError('');
    setDiscoveryPhase('idle');
  }

  function handleSourceUrlChange(nextValue: string) {
    setSourceUrl(nextValue);

    if (discoveryTimerRef.current != null) {
      window.clearTimeout(discoveryTimerRef.current);
      discoveryTimerRef.current = null;
    }

    const input = nextValue.trim();
    if (!input) {
      setDiscovery(null);
      setDiscoveryError('');
      setDiscoveryPhase('idle');
      return;
    }

    setDiscovery(null);
    setDiscoveryError('');
    setDiscoveryPhase('detecting');

    discoveryTimerRef.current = window.setTimeout(() => {
      if (!isDomainLike(input)) {
        setDiscoveryPhase('error');
        setDiscoveryError(t('sources.noFeedFoundShort'));
        return;
      }

      setDiscovery(buildDiscovery(input));
      setDiscoveryPhase('found');
      discoveryTimerRef.current = null;
    }, 700);
  }

  function alreadySubscribed(candidate: Discovery) {
    const candidateUrl = normalizeComparableUrl(candidate.url);
    return visibleSources.some((source) => normalizeComparableUrl(source.url) === candidateUrl);
  }

  async function subscribe(candidate: Discovery) {
    if (alreadySubscribed(candidate)) {
      showToast(t('sources.toastDuplicate'));
      return;
    }

    try {
      const source = await createSource.mutateAsync(candidate.url);
      resetDiscovery();
      showToast(t('sources.toastSubscribed', { title: source.title || candidate.title }));
      refreshSource.mutate(source.id);
    } catch (error) {
      const raw = error instanceof ApiError || error instanceof Error ? error.message : '';
      setDiscoveryPhase('error');
      setDiscoveryError(raw || t('sources.noFeedFoundShort'));
    }
  }

  async function subscribeStarterPack(starter: typeof STARTER_PACKS[number]) {
    const candidate = buildDiscovery(starter.url);
    candidate.title = starter.title;
    await subscribe(candidate);
  }

  async function refreshOne(source: SourceViewModel) {
    setRefreshingIds((previous) => new Set(previous).add(source.id));
    try {
      await refreshSource.mutateAsync(source.id);
      setRefreshedAt((previous) => {
        const next = new Map(previous);
        next.set(source.id, Date.now());
        return next;
      });
    } catch (error) {
      const detail = error instanceof Error ? error.message : t('sources.toastRefreshFailed');
      showToast(detail);
    } finally {
      setRefreshingIds((previous) => {
        const next = new Set(previous);
        next.delete(source.id);
        return next;
      });
    }
  }

  function refreshAll() {
    const targets = filteredSources;
    showToast(t('sources.toastRefreshing', { count: targets.length }));
    targets.forEach((source, index) => {
      window.setTimeout(() => {
        void refreshOne(source);
      }, 120 * index);
    });
  }

  async function handleDelete(source: SourceViewModel) {
    try {
      await deleteSource.mutateAsync(source.id);
      setDeletedSourceIds((previous) => new Set(previous).add(source.id));
      showToast(t('sources.toastUnsubscribed', { title: source.title }));
    } catch (error) {
      const detail = error instanceof Error ? error.message : t('sources.delete');
      showToast(detail);
    }
  }

  async function handleImport(file: File) {
    handledImportJobId.current = null;

    try {
      const raw = await file.text();
      const response = await apiFetch<{ job_id: string }>('/api/sources/import', {
        method: 'POST',
        headers: { 'Content-Type': 'text/x-opml; charset=utf-8' },
        body: raw,
      });
      startSourceImport(response.job_id, file.name);
      showToast(t('sources.importingMessage'));
    } catch (error) {
      const detail = error instanceof Error ? error.message : t('sources.importFailedMessage');
      clearSourceImport();
      showToast(detail);
    }
  }

  async function handleExport() {
    const response = await fetch('/api/sources/export', { credentials: 'include' });
    if (!response.ok) {
      showToast(t('sources.exportFailed'));
      return;
    }

    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'xreader-sources.opml';
    anchor.click();
    URL.revokeObjectURL(url);
    showToast(t('sources.exportDone'));
  }

  function clearFilters() {
    setStatusFilter('all');
    setFilterQuery('');
  }

  const rootClassName = [
    styles.sourcesPage,
    tone === 'editorial' ? styles.editorial : '',
    showFavicons ? '' : styles.noFavicons,
  ].filter(Boolean).join(' ');
  const addbarClassName = [
    styles.addbarRow,
    discoveryPhase === 'found' ? styles.addbarFound : '',
    discoveryPhase === 'error' ? styles.addbarError : '',
  ].filter(Boolean).join(' ');

  return (
    <main className={rootClassName}>
      <div className={styles.inner}>
        <Link href="/" className={styles.backLink}>
          <ChevronLeft size={14} strokeWidth={1.5} />
          <span>{t('sources.backHome')}</span>
        </Link>

        <header className={styles.headRow}>
          <div>
            <h1 className={styles.pageTitle}>{t('sources.title')}</h1>
            <p className={styles.subtitle}>{t('sources.subtitle')}</p>
          </div>

          <div className={styles.headRight}>
            <div className={styles.stat}>
              <div className={styles.statNumber}>{counts.all}</div>
              <div className={styles.statLabel}>{t('sources.subscriptionStat')}</div>
            </div>
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <div className={styles.statNumber}>{attentionCount}</div>
              <div className={styles.statLabel}>{t('sources.attentionStat')}</div>
            </div>
            <button type="button" className={styles.buttonSecondary} onClick={() => void handleExport()}>
              <Download size={14} strokeWidth={1.5} />
              {t('sources.export')}
            </button>
          </div>
        </header>

        {!isGuest && <section className={styles.addbar} aria-label={t('sources.addTitle')}>
          <div className={addbarClassName}>
            <Search className={styles.addbarIcon} size={18} strokeWidth={1.5} />
            <input
              className={styles.addbarInput}
              value={sourceUrl}
              placeholder={t('sources.addPlaceholderUnified')}
              spellCheck={false}
              autoComplete="off"
              onChange={(event) => handleSourceUrlChange(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Escape') resetDiscovery();
                if (event.key === 'Enter' && discoveryPhase === 'found' && discovery) {
                  void subscribe(discovery);
                }
              }}
            />
            {discoveryPhase === 'detecting' ? (
              <RefreshCw aria-label={t('sources.discoveryDetectingAria')} className={styles.spinningIcon} size={16} strokeWidth={1.5} />
            ) : null}
            {sourceUrl && discoveryPhase !== 'detecting' ? (
              <button type="button" className={styles.addbarClear} aria-label="Clear" onClick={resetDiscovery}>
                <X size={12} strokeWidth={1.5} />
              </button>
            ) : null}
            <span className={styles.addbarDivider} aria-hidden="true" />
            <button
              type="button"
              className={styles.opmlButton}
              onClick={() => fileInputRef.current?.click()}
            >
              <Upload size={14} strokeWidth={1.5} />
              {t('sources.opmlInline')}
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".opml,.xml,application/xml,text/xml"
              hidden
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void handleImport(file);
                event.target.value = '';
              }}
            />
          </div>

          {discoveryPhase === 'found' && discovery ? (
            <div className={styles.discovery}>
              <div className={styles.discoveryLeft}>
                <span className={styles.discoveryFavicon} aria-hidden="true">
                  <span className={styles.faviconImage} style={{ backgroundImage: `url("${faviconUrl(discovery.site)}")` }} />
                </span>
                <div className={styles.discoveryMeta}>
                  <div className={styles.discoveryTitle}>{discovery.title}</div>
                  <div className={styles.discoveryUrl}>{discovery.url}</div>
                </div>
                <span className={styles.discoveryTag}>{t('sources.discoveryItemCount', { count: discovery.items })}</span>
              </div>
              <div className={styles.discoveryActions}>
                <button type="button" className={styles.buttonGhost} onClick={resetDiscovery}>
                  {t('sources.cancel')}
                </button>
                <button
                  type="button"
                  className={styles.buttonPrimary}
                  onClick={() => void subscribe(discovery)}
                  disabled={createSource.isPending}
                >
                  {createSource.isPending ? (
                    <span className="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
                  ) : (
                    <Plus size={14} strokeWidth={1.5} />
                  )}
                  {createSource.isPending ? t('sources.subscribing') : t('sources.subscribe')}
                </button>
              </div>
            </div>
          ) : null}

          {discoveryPhase === 'error' ? (
            <div className={`${styles.discovery} ${styles.discoveryError}`}>
              <span className={styles.discoveryErrorText}>{discoveryError || t('sources.noFeedFoundShort')}</span>
              <button type="button" className={styles.buttonGhost} onClick={resetDiscovery}>
                {t('sources.dismiss')}
              </button>
            </div>
          ) : null}
        </section>}

        {importRunning ? (
          <section className={styles.importSheet}>
            <div className={styles.importHead}>
              <div>
                <div className={styles.importTitle}>{t('sources.importingFile', { filename: sourceImportJob?.fileName || 'subscriptions.opml' })}</div>
                <div className={styles.importSub}>
                  {t('sources.importProgressFeeds', {
                    done: importCompleted,
                    total: importTotal,
                    state: importFailed ? t('sources.importFailed') : importDone ? t('sources.importDoneState') : t('sources.importPolling'),
                  })}
                </div>
              </div>
              {importDone || importFailed ? (
                <button
                  type="button"
                  className={styles.buttonGhost}
                  onClick={clearSourceImport}
                >
                  {t('sources.dismiss')}
                </button>
              ) : null}
            </div>
            <div className={styles.importBar}>
              <div className={styles.importBarFill} style={{ width: `${importProgress || 12}%` }} />
            </div>
          </section>
        ) : null}

        {sourceRows.length > 0 ? (
          <section className={styles.listToolbar}>
            <div className={styles.filters}>
              {STATUS_FILTERS.map((filter) => {
                const count = counts[filter.id];
                const active = statusFilter === filter.id;
                return (
                  <button
                    key={filter.id}
                    type="button"
                    aria-label={`${t(filter.labelKey)} ${count}`}
                    className={[
                      styles.chip,
                      filter.id === 'error' ? styles.chipError : '',
                      active ? styles.chipActive : '',
                      active && filter.id === 'error' ? styles.chipErrorActive : '',
                    ].filter(Boolean).join(' ')}
                    onClick={() => setStatusFilter(filter.id)}
                  >
                    <span>{t(filter.labelKey)}</span>
                    <span className={styles.chipCount}>{count}</span>
                  </button>
                );
              })}
            </div>

            <div className={styles.toolbarRight}>
              <label className={styles.searchMini}>
                <Search size={13} strokeWidth={1.5} />
                <input
                  value={filterQuery}
                  placeholder={t('sources.filterPlaceholder')}
                  onChange={(event) => setFilterQuery(event.target.value)}
                />
              </label>
              <select
                className={styles.sortSelect}
                value={sortMode}
                onChange={(event) => setSortMode(event.target.value as SortMode)}
              >
                <option value="recent">{t('sources.sortRecent')}</option>
                <option value="title">{t('sources.sortTitle')}</option>
                <option value="items">{t('sources.sortItems')}</option>
              </select>
              <button type="button" className={styles.buttonSecondary} onClick={refreshAll}>
                <RefreshCw size={14} strokeWidth={1.5} />
                {t('sources.refreshAll')}
              </button>
            </div>
          </section>
        ) : null}

        {isLoading ? (
          <div className={styles.noMatch}>{t('sources.loading')}</div>
        ) : sourceRows.length === 0 ? (
          <section className={styles.empty}>
            <div className={styles.emptyGlyph}>
              <Search size={20} strokeWidth={1.5} />
            </div>
            <div className={styles.emptyTitle}>{t('sources.noSubscriptionsTitle')}</div>
            <p className={styles.emptySub}>{t('sources.noSubscriptionsDescription')}</p>
            <div className={styles.suggestions}>
              {STARTER_PACKS.map((starter) => (
                <button
                  key={starter.url}
                  type="button"
                  className={styles.suggestion}
                  onClick={() => void subscribeStarterPack(starter)}
                >
                  <span
                    className={styles.suggestionFavicon}
                    style={{ backgroundImage: `url("${faviconUrl(getHostFromUrl(starter.url))}")` }}
                    aria-hidden="true"
                  />
                  <span>{starter.title}</span>
                  <Plus size={12} strokeWidth={1.5} />
                </button>
              ))}
            </div>
          </section>
        ) : filteredSources.length === 0 ? (
          <section className={styles.noMatch}>
            <div className={styles.noMatchText}>{t('sources.noMatches')}</div>
            <button type="button" className={styles.buttonText} onClick={clearFilters}>
              {t('sources.clearFilters')}
            </button>
          </section>
        ) : viewMode === 'cards' ? (
          <section className={styles.grid}>
            {filteredSources.map((source) => (
              <SourceCard
                key={source.id}
                source={source}
                refreshing={refreshingIds.has(source.id)}
                onRefresh={(target) => void refreshOne(target)}
                onDelete={(target) => void handleDelete(target)}
                isGuest={!!isGuest}
                t={t}
              />
            ))}
          </section>
        ) : (
          <section className={styles.list}>
            {filteredSources.map((source) => (
              <SourceRow
                key={source.id}
                source={source}
                density={density}
                refreshing={refreshingIds.has(source.id)}
                onRefresh={(target) => void refreshOne(target)}
                onDelete={(target) => void handleDelete(target)}
                isGuest={!!isGuest}
                t={t}
              />
            ))}
          </section>
        )}

        <footer className={styles.footer}>
          <span>{t('sources.footerSummary', { count: sourceRows.length, time: formatRelativeTime(latestSync, t) })}</span>
        </footer>
      </div>

      <button
        type="button"
        className={`${styles.buttonSecondary} ${styles.tweaksButton}`}
        onClick={() => setIsTweaksOpen((open) => !open)}
      >
        <Settings2 size={14} strokeWidth={1.5} />
        {t('sources.tweaks')}
      </button>

      {isTweaksOpen ? (
        <aside className={styles.tweaksPanel}>
          <div className={styles.tweaksHeader}>
            <span>{t('sources.tweaks')}</span>
            <button type="button" className={styles.iconButton} aria-label={t('sources.dismiss')} onClick={() => setIsTweaksOpen(false)}>
              <X size={14} strokeWidth={1.5} />
            </button>
          </div>

          <div className={styles.tweakGroup}>
            <div className={styles.tweakLabel}>{t('sources.tweakDensity')}</div>
            <div className={styles.segmented}>
              <SegmentButton value="compact" current={density} onSelect={setDensity}>{t('sources.tweakCompact')}</SegmentButton>
              <SegmentButton value="comfortable" current={density} onSelect={setDensity}>{t('sources.tweakComfortable')}</SegmentButton>
            </div>
          </div>

          <div className={styles.tweakGroup}>
            <div className={styles.tweakLabel}>{t('sources.tweakView')}</div>
            <div className={styles.segmented}>
              <SegmentButton value="list" current={viewMode} onSelect={setViewMode}>
                <List size={13} strokeWidth={1.5} />
                {t('sources.tweakList')}
              </SegmentButton>
              <SegmentButton value="cards" current={viewMode} onSelect={setViewMode}>
                <Grid2X2 size={13} strokeWidth={1.5} />
                {t('sources.tweakCards')}
              </SegmentButton>
            </div>
          </div>

          <div className={styles.tweakGroup}>
            <div className={styles.tweakLabel}>{t('sources.tweakTone')}</div>
            <div className={styles.segmented}>
              <SegmentButton value="quiet" current={tone} onSelect={setTone}>{t('sources.tweakQuiet')}</SegmentButton>
              <SegmentButton value="editorial" current={tone} onSelect={setTone}>{t('sources.tweakEditorial')}</SegmentButton>
            </div>
          </div>

          <div className={styles.tweakGroup}>
            <button type="button" className={styles.toggleRow} onClick={() => setShowFavicons((value) => !value)}>
              <span>{t('sources.tweakFavicons')}</span>
              <span className={`${styles.toggle} ${showFavicons ? styles.toggleOn : ''}`} />
            </button>
          </div>
        </aside>
      ) : null}

      {toast ? <div className={styles.toast}>{toast}</div> : null}
    </main>
  );
}

export default SourcesPage;
