'use client';

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { applyArticleStateChange } from '@/lib/article-state-cache';
import { subscribe } from '@/lib/broadcast';

export function useBroadcastSync() {
  const queryClient = useQueryClient();

  useEffect(() => {
    return subscribe((msg) => {
      applyArticleStateChange(queryClient, msg);
    });
  }, [queryClient]);
}
