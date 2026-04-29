'use client';

import { ChangeEvent } from '@/lib/api';
import type { MetricInfo } from '@/components/promql-modal';
import TimelineEvent from './timeline-event';
import DeploymentStoryCard from './deployment-story';

export type ViewMode = 'story' | 'classic';

interface TimelineProps {
  events: ChangeEvent[];
  viewMode?: ViewMode;
  selectedMetrics?: Set<string>;
  availableMetrics?: MetricInfo[];
}

export default function Timeline({ events, viewMode = 'story', selectedMetrics, availableMetrics }: TimelineProps) {
  return (
    <div className="flex flex-col space-y-8 relative">
      <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-gray-200"></div>
      {events.map((event) =>
        viewMode === 'story' && event.linked_intent ? (
          <DeploymentStoryCard key={event.id} event={event} selectedMetrics={selectedMetrics} availableMetrics={availableMetrics} />
        ) : (
          <TimelineEvent key={event.id} event={event} selectedMetrics={selectedMetrics} availableMetrics={availableMetrics} />
        )
      )}
    </div>
  );
}
