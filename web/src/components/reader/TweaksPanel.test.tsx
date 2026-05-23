import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TweaksPanel } from './TweaksPanel';
import { useUIStore } from '@/stores/useUIStore';

beforeEach(() => {
  useUIStore.setState({
    layout: 'classic',
    focusMode: false,
    density: 'comfortable',
    fontSize: 17,
    accentColor: 'blue',
    theme: 'system',
    nativeLanguage: 'zh-CN',
  });
});

test('switches to focus layout from tweaks panel', async () => {
  const user = userEvent.setup();
  render(<TweaksPanel externalOpen />);

  expect(screen.getByText('阅读设置')).toBeInTheDocument();
  expect(screen.getByText('版式')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '专注' }));

  expect(useUIStore.getState().layout).toBe('focus');
  expect(useUIStore.getState().focusMode).toBe(true);
});

test('switches back to wide layout and exits focus mode', async () => {
  const user = userEvent.setup();
  useUIStore.setState({ layout: 'focus', focusMode: true });

  render(<TweaksPanel externalOpen />);

  await user.click(screen.getByRole('button', { name: '宽屏' }));

  expect(useUIStore.getState().layout).toBe('wide');
  expect(useUIStore.getState().focusMode).toBe(false);
});

test('changes theme from tweaks panel', async () => {
  const user = userEvent.setup();
  render(<TweaksPanel externalOpen />);

  expect(screen.getByText('主题')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '深色' }));

  expect(useUIStore.getState().theme).toBe('dark');
});
