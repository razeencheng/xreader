import { useI18n } from '@/lib/i18n';

interface Props {
  text: string;
}

function splitSummary(text: string) {
  return text
    .split(/\s*[·•①②③④⑤⑥⑦⑧⑨⑩；;]\s*/)
    .map((point) => point.trim())
    .filter(Boolean);
}

export function KeyPointsCallout({ text }: Props) {
  const { t } = useI18n();
  if (!text) return null;

  const points = splitSummary(text);

  return (
    <div className="mb-[30px] rounded-r-[10px] border-l-[3px] border-[var(--accent)] bg-[var(--callout-bg)] px-[18px] py-4">
      <div className="mb-2 text-[10px] font-semibold tracking-[0.16em] text-[var(--text-3)]">
        {t('reader.keyPoints')}：
      </div>
      {points.length > 1 ? (
        <ul className="pl-5 text-[0.9em] leading-relaxed text-[var(--text-2)]">
          {points.map((point, index) => (
            <li key={index} className="mb-[5px]">
              {point}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-[0.9em] leading-relaxed text-[var(--text-2)]">{text}</p>
      )}
    </div>
  );
}
