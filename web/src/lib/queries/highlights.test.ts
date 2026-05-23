import { updateHighlightNote } from './highlights';
import { apiFetch } from '@/lib/api-client';

vi.mock('@/lib/api-client', () => ({
  apiFetch: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(apiFetch).mockReset();
});

test('updates highlight notes through the note endpoint', async () => {
  vi.mocked(apiFetch).mockResolvedValue(undefined);

  await updateHighlightNote(12, 'remember this');

  expect(apiFetch).toHaveBeenCalledWith('/api/highlights/12/note', {
    method: 'PUT',
    body: JSON.stringify({ note: 'remember this' }),
  });
});
