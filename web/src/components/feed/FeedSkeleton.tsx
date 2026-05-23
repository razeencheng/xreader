'use client';

export function FeedSkeleton() {
  return (
    <div className="w-full px-4 py-1.5 md:px-7">
      <div className="divide-y divide-[var(--border-default)]">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="py-6 animate-pulse">
            <div className="flex items-center gap-3 mb-3">
              <div className="h-4 w-16 bg-[var(--bg-surface)] rounded" />
              <div className="h-3 w-24 bg-[var(--bg-surface)] rounded opacity-60" />
            </div>
            <div className="h-6 w-3/4 bg-[var(--bg-surface)] rounded mb-2" />
            <div className="h-4 w-1/2 bg-[var(--bg-surface)] rounded opacity-60" />
          </div>
        ))}
      </div>
    </div>
  );
}

export function CompactSkeleton() {
  return (
    <div className="w-full px-4 py-1.5 md:px-7">
      <div className="divide-y divide-[var(--border-default)]">
        {[...Array(10)].map((_, i) => (
          <div key={i} className="py-3 flex items-center gap-3 animate-pulse">
            <div className="h-4 w-4 rounded-full bg-[var(--bg-surface)]" />
            <div className="h-4 w-12 bg-[var(--bg-surface)] rounded" />
            <div className="h-4 w-full bg-[var(--bg-surface)] rounded" />
          </div>
        ))}
      </div>
    </div>
  );
}
