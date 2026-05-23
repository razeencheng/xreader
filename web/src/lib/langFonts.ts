const LANG_FONTS: Record<string, string> = {
  ja: "'Hiragino Mincho ProN', Georgia, serif",
  zh: "'Source Han Serif SC', 'Noto Serif CJK SC', 'Songti SC', Georgia, serif",
  'zh-cn': "'Source Han Serif SC', 'Noto Serif CJK SC', 'Songti SC', Georgia, serif",
  'zh-tw': "'Source Han Serif TC', 'Noto Serif CJK TC', Georgia, serif",
  ko: "'Nanum Myeongjo', Georgia, serif",
};

export function fontForLang(lang: string): string {
  return LANG_FONTS[lang.toLowerCase()] ?? "'Iowan Old Style', Charter, Georgia, serif";
}
