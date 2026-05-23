import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useAuthStore } from '@/stores/useAuthStore';
import { useUIStore } from '@/stores/useUIStore';
import { MobileTopBar } from './ResponsiveAppNav';

const push = vi.fn();
const usePathname = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push }),
  usePathname: () => usePathname(),
}));

beforeEach(() => {
  push.mockReset();
  usePathname.mockReturnValue('/');
  useAuthStore.setState({
    user: {
      id: 1,
      github_username: 'jin',
      role: 'user',
      native_language: 'zh-CN',
      density_pref: 'comfortable',
      theme_pref: 'light',
    },
    isLoading: false,
  });
  useUIStore.setState({
    currentView: 'today',
    nativeLanguage: 'zh-CN',
    focusMode: false,
    selectedSourceId: null,
  });
});

test('MobileTopBar exposes one current-view menu on list pages', async () => {
  const user = userEvent.setup();

  render(<MobileTopBar focusMode={false} />);

  expect(screen.queryByRole('navigation', { name: '移动端主导航' })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '今日' })).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '全部' })).not.toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '今日' }));

  expect(screen.getByText('视图')).toBeInTheDocument();
  expect(screen.getByText('工具')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '收藏' }));

  expect(useUIStore.getState().currentView).toBe('starred');
});

test('MobileTopBar links to the highlights and notes page from tools', async () => {
  const user = userEvent.setup();

  render(<MobileTopBar focusMode={false} />);

  await user.click(screen.getByRole('button', { name: '今日' }));
  await user.click(screen.getByRole('button', { name: '我的高亮' }));

  expect(push).toHaveBeenCalledWith('/highlights');
});

test('MobileTopBar shows normal navigation on non-list pages (e.g. /settings)', () => {
  usePathname.mockReturnValue('/settings');

  render(<MobileTopBar focusMode={false} />);

  expect(screen.queryByRole('navigation', { name: '移动端主导航' })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /xReader/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '更多' })).toBeInTheDocument();
});
