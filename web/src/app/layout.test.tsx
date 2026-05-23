vi.mock('next/font/local', () => ({
  default: () => ({ variable: 'font-local' }),
}));

import { viewport } from './layout';

test('mobile browser chrome follows the app surface color', () => {
  expect(viewport).toMatchObject({
    themeColor: '#f9f7f1',
    viewportFit: 'cover',
  });
});
