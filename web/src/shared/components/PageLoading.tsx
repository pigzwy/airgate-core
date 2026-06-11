import { ProgressBar } from '@heroui/react';

interface TopLoadingLineProps {
  active?: boolean;
}

export function TopLoadingLine({ active = true }: TopLoadingLineProps) {
  if (!active) return null;

  return (
    <ProgressBar.Root
      aria-label="Loading"
      className="pointer-events-none fixed inset-x-0 top-0 z-[80] h-1 gap-0"
      color="accent"
      isIndeterminate
      size="sm"
      style={{ gridTemplateAreas: '"track"', gridTemplateColumns: '1fr' }}
    >
      <ProgressBar.Track className="h-full rounded-none bg-transparent">
        <ProgressBar.Fill className="rounded-none" />
      </ProgressBar.Track>
    </ProgressBar.Root>
  );
}

export function PageLoading() {
  return <TopLoadingLine />;
}

export function FullPageLoading() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <TopLoadingLine />
    </div>
  );
}

export function ChatPageLoading() {
  return (
    <div className="h-full min-h-0 bg-background text-foreground">
      <TopLoadingLine />
    </div>
  );
}
