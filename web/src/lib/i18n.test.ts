import { describe, expect, test } from 'vitest';
import { getUILanguage, translate } from './i18n';

describe('i18n', () => {
  test('normalizes BCP-47 native languages to UI dictionaries', () => {
    expect(getUILanguage('zh-CN')).toBe('zh-CN');
    expect(getUILanguage('zh-TW')).toBe('zh-TW');
    expect(getUILanguage('en-US')).toBe('en');
    expect(getUILanguage('ja-JP')).toBe('ja');
    expect(getUILanguage('ko-KR')).toBe('ko');
    expect(getUILanguage('pt-BR')).toBe('pt');
  });

  test('translates common navigation labels and interpolated messages', () => {
    expect(translate('zh-CN', 'nav.today')).toBe('今日');
    expect(translate('zh-TW', 'nav.settings')).toBe('設定');
    expect(translate('en-US', 'nav.today')).toBe('Today');
    expect(translate('ja-JP', 'feed.allRead')).toBe('すべて既読');
    expect(translate('ja-JP', 'settings.save')).toBe('設定を保存');
    expect(translate('es-ES', 'sources.findAndAdd')).toBe('Buscar y añadir');
    expect(translate('zh-CN', 'feed.bulkReadNotice', { scope: '当前视图', count: 2 })).toBe('已将当前视图 2 篇标为已读');
    expect(translate('en-US', 'feed.bulkReadNotice', { scope: 'current view', count: 2 })).toBe('Marked 2 articles in current view as read');
  });

  test('falls back to English for unknown languages and keys for missing messages', () => {
    expect(translate('it-IT', 'nav.settings')).toBe('Settings');
    expect(translate('zh-CN', 'missing.key')).toBe('missing.key');
  });
});
