import { render, screen } from '@testing-library/react';
import { KeyPointsCallout } from './KeyPointsCallout';

test('KeyPointsCallout renders split summary items', () => {
  render(<KeyPointsCallout text="① aaa ② bbb ③ ccc" />);
  expect(screen.getByText(/aaa/)).toBeInTheDocument();
  expect(screen.getByText(/bbb/)).toBeInTheDocument();
  expect(screen.getByText(/ccc/)).toBeInTheDocument();
});

test('KeyPointsCallout renders plain text when no separator exists', () => {
  render(<KeyPointsCallout text="single summary" />);
  expect(screen.getByText('single summary')).toBeInTheDocument();
});
