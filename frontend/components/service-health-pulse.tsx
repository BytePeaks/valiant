'use client';

import type { ServiceHealth } from '@/lib/api';

const STATUS_CONFIG = {
  healthy: {
    dot: 'bg-emerald-500',
    ring: 'ring-emerald-200',
    label: 'Healthy',
    title: (level: string, score: number) => `${level} impact (score: ${score.toFixed(2)})`,
  },
  warning: {
    dot: 'bg-amber-400',
    ring: 'ring-amber-200',
    label: 'Warning',
    title: (level: string, score: number) => `${level} impact (score: ${score.toFixed(2)})`,
  },
  degraded: {
    dot: 'bg-red-500',
    ring: 'ring-red-200',
    label: 'Degraded',
    title: (level: string, score: number) => `${level} impact (score: ${score.toFixed(2)})`,
  },
  unknown: {
    dot: 'bg-gray-300',
    ring: 'ring-gray-100',
    label: 'No data',
    title: () => 'No analysis available',
  },
};

interface ServiceHealthPulseProps {
  health: ServiceHealth | undefined;
  showLabel?: boolean;
  size?: 'sm' | 'md';
}

export default function ServiceHealthPulse({ health, showLabel = false, size = 'sm' }: ServiceHealthPulseProps) {
  const status = health?.status ?? 'unknown';
  const config = STATUS_CONFIG[status];
  const dotSize = size === 'sm' ? 'w-2 h-2' : 'w-3 h-3';
  const title = health
    ? config.title(health.impact_level, health.impact_score)
    : config.title('', 0);

  return (
    <span
      className="inline-flex items-center gap-1.5 shrink-0"
      title={title}
    >
      <span className={`relative inline-flex ${dotSize}`}>
        {(status === 'warning' || status === 'degraded') && (
          <span className={`animate-ping absolute inline-flex h-full w-full rounded-full ${config.dot} opacity-60`} />
        )}
        <span className={`relative inline-flex rounded-full ${dotSize} ${config.dot} ring-2 ${config.ring}`} />
      </span>
      {showLabel && (
        <span className="text-xs font-semibold">{config.label}</span>
      )}
    </span>
  );
}
