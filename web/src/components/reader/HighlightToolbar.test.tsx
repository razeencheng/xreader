import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HighlightToolbar } from './HighlightToolbar';
import { createHighlight } from '@/lib/queries/highlights';

const highlightQueries = vi.hoisted(() => ({
  createHighlight: vi.fn(),
}));

vi.mock('@/lib/queries/highlights', () => ({
  createHighlight: highlightQueries.createHighlight,
}));

beforeEach(() => {
  highlightQueries.createHighlight.mockReset();
});

test('creates a highlight with note through an inline editor instead of a browser prompt', async () => {
  const promptSpy = vi.spyOn(window, 'prompt').mockImplementation(() => 'native prompt');
  highlightQueries.createHighlight.mockResolvedValue({
    id: 1,
    article_id: 7,
    layer: 'original',
    paragraph_index: 0,
    text_start_offset: 0,
    text_end_offset: 5,
    quoted_text: 'Hello',
    created_at: '2026-04-26T00:00:00Z',
  });
  const user = userEvent.setup();

  render(
    <>
      <p data-layer="original" data-paragraph-index="0">Hello world</p>
      <HighlightToolbar articleId={7} />
    </>,
  );

  const paragraph = screen.getByText('Hello world');
  const range = document.createRange();
  range.setStart(paragraph.firstChild!, 0);
  range.setEnd(paragraph.firstChild!, 5);
  Object.defineProperty(range, 'getBoundingClientRect', {
    value: () => ({
    bottom: 20,
    height: 10,
    left: 20,
    right: 80,
    top: 10,
    width: 60,
    x: 20,
    y: 10,
    toJSON: () => ({}),
    }),
  });
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
  fireEvent.pointerUp(document);

  await user.click(screen.getByRole('button', { name: '高亮并添加备注' }));

  expect(promptSpy).not.toHaveBeenCalled();
  expect(screen.getByRole('button', { name: '高亮' })).toHaveClass('min-h-11', 'min-w-11');
  expect(screen.getByRole('button', { name: '高亮并添加备注' })).toHaveClass('min-h-11', 'min-w-11');
  expect(screen.getByRole('dialog', { name: '高亮备注' })).toBeInTheDocument();

  await user.type(screen.getByLabelText('备注'), 'note text');
  const saveButton = screen.getByRole('button', { name: '保存' });
  expect(saveButton).toHaveClass('min-h-11');
  await user.click(saveButton);

  expect(createHighlight).toHaveBeenCalledWith({
    article_id: 7,
    layer: 'original',
    paragraph_index: 0,
    text_start_offset: 0,
    text_end_offset: 5,
    quoted_text: 'Hello',
    note: 'note text',
  });
  promptSpy.mockRestore();
});
