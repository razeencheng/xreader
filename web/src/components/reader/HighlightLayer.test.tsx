import { computeAnchor } from './highlightAnchor';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HighlightLayer } from './HighlightLayer';
import { updateHighlightNote } from '@/lib/queries/highlights';

const highlightQueries = vi.hoisted(() => ({
  fetchHighlights: vi.fn(),
  updateHighlightNote: vi.fn(),
  deleteHighlight: vi.fn(),
  createHighlight: vi.fn(),
}));

vi.mock('@/lib/queries/highlights', () => ({
  fetchHighlights: highlightQueries.fetchHighlights,
  updateHighlightNote: highlightQueries.updateHighlightNote,
  deleteHighlight: highlightQueries.deleteHighlight,
  createHighlight: highlightQueries.createHighlight,
}));

beforeEach(() => {
  highlightQueries.fetchHighlights.mockReset();
  highlightQueries.updateHighlightNote.mockReset();
  highlightQueries.deleteHighlight.mockReset();
  highlightQueries.createHighlight.mockReset();
});

test('computes offsets relative to paragraph text', () => {
  const p = document.createElement('p');
  p.setAttribute('data-paragraph-index', '3');
  p.textContent = 'Hello world friend';
  document.body.appendChild(p);

  const range = document.createRange();
  range.setStart(p.firstChild!, 6);
  range.setEnd(p.firstChild!, 11);

  const anchor = computeAnchor(range);
  expect(anchor).toEqual({
    layer: 'original',
    paragraph_index: 3,
    text_start_offset: 6,
    text_end_offset: 11,
    quoted_text: 'world',
  });

  document.body.removeChild(p);
});

test('returns null for empty selection', () => {
  const p = document.createElement('p');
  p.setAttribute('data-paragraph-index', '0');
  p.textContent = 'Hello';
  document.body.appendChild(p);

  const range = document.createRange();
  range.setStart(p.firstChild!, 3);
  range.setEnd(p.firstChild!, 3);

  expect(computeAnchor(range)).toBeNull();

  document.body.removeChild(p);
});

test('returns null when no paragraph element found', () => {
  const div = document.createElement('div');
  div.textContent = 'No paragraph';
  document.body.appendChild(div);

  const range = document.createRange();
  range.setStart(div.firstChild!, 0);
  range.setEnd(div.firstChild!, 2);

  expect(computeAnchor(range)).toBeNull();

  document.body.removeChild(div);
});

test('renders translation highlights on the translation layer with the same paragraph index', async () => {
  highlightQueries.fetchHighlights.mockResolvedValue([
    {
      id: 42,
      article_id: 7,
      layer: 'translation',
      paragraph_index: 0,
      text_start_offset: 0,
      text_end_offset: 2,
      quoted_text: '译文',
      created_at: '2026-04-26T00:00:00Z',
    },
  ]);

  const { container } = render(
    <HighlightLayer articleId={7}>
      <div data-layer="original" data-paragraph-index="0">Original text</div>
      <div data-layer="translation" data-paragraph-index="0">译文内容</div>
    </HighlightLayer>,
  );

  await waitFor(() => expect(container.querySelector('mark[data-highlight-id="42"]')).toBeInTheDocument());

  const originalLayer = container.querySelector('[data-layer="original"]');
  const translationLayer = container.querySelector('[data-layer="translation"]');
  expect(originalLayer?.querySelector('mark')).toBeNull();
  expect(translationLayer?.querySelector('mark[data-highlight-id="42"]')).toHaveTextContent('译文');
});

test('opens an inline note editor for existing highlights instead of a browser prompt', async () => {
  const promptSpy = vi.spyOn(window, 'prompt').mockImplementation(() => 'native prompt');
  highlightQueries.fetchHighlights.mockResolvedValue([
    {
      id: 43,
      article_id: 7,
      layer: 'original',
      paragraph_index: 0,
      text_start_offset: 0,
      text_end_offset: 5,
      quoted_text: 'Hello',
      note: 'old note',
      created_at: '2026-04-26T00:00:00Z',
    },
  ]);
  highlightQueries.updateHighlightNote.mockResolvedValue(undefined);

  const user = userEvent.setup();
  render(
    <HighlightLayer articleId={7}>
      <div data-layer="original" data-paragraph-index="0">Hello world</div>
    </HighlightLayer>,
  );

  const mark = await screen.findByText('Hello');
  await user.click(mark);

  expect(promptSpy).not.toHaveBeenCalled();
  expect(screen.getByRole('dialog', { name: '高亮备注' })).toBeInTheDocument();

  const textarea = screen.getByLabelText('备注');
  await user.clear(textarea);
  await user.type(textarea, 'new note');
  const saveButton = screen.getByRole('button', { name: '保存' });
  expect(saveButton).toHaveClass('min-h-11');
  await user.click(saveButton);

  expect(updateHighlightNote).toHaveBeenCalledWith(43, 'new note');
  promptSpy.mockRestore();
});
