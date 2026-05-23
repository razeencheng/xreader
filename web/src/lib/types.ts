export interface ArticleItem {
  id: number;
  source_id: number;
  title: string;
  link: string;
  language: string;
  author?: string;
  published_at?: string;
  title_translated?: string;
  summary?: string;
  source_title?: string;
  source_icon_url?: string;
  word_count?: number;
  is_read?: boolean;
  is_starred?: boolean;
}

export type Article = ArticleItem;

export interface Source {
  id: number;
  title: string;
  url: string;
  icon_url?: string | null;
  last_fetched_at: string | null;
  last_success_at?: string | null;
  consecutive_fails: number;
  health: string;
  category: string;
  unread_count?: number;
}

export interface ArticleListResponse {
  items: ArticleItem[];
  next_cursor?: string;
  counts?: {
    unread: number;
    all: number;
    read: number;
  };
}

export type ArticleTab = 'today' | 'stream' | 'starred';
