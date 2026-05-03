'use client';

import { Layers } from 'lucide-react';
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

const CONCURRENT_WINDOW_MS = 30 * 60 * 1000;

function groupConcurrentEvents(events: ChangeEvent[]): ChangeEvent[][] {
  if (events.length === 0) return [];
  // Events arrive newest-first; group by proximity to the newest event in each group.
  const groups: ChangeEvent[][] = [];
  let current: ChangeEvent[] = [events[0]];
  // Anchor to the newest timestamp in the current group (first element, since DESC order).
  let anchorMs = new Date(events[0].timestamp).getTime();

  for (let i = 1; i < events.length; i++) {
    const ms = new Date(events[i].timestamp).getTime();
    if (anchorMs - ms <= CONCURRENT_WINDOW_MS) {
      current.push(events[i]);
    } else {
      groups.push(current);
      current = [events[i]];
      anchorMs = ms;
    }
  }
  groups.push(current);
  return groups;
}

function fmt(d: Date) {
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export default function Timeline({ events, viewMode = 'story', selectedMetrics, availableMetrics }: TimelineProps) {
  const groups = groupConcurrentEvents(events);

  const renderEvent = (event: ChangeEvent) =>
    viewMode === 'story' && event.linked_intent ? (
      <DeploymentStoryCard key={event.id} event={event} selectedMetrics={selectedMetrics} availableMetrics={availableMetrics} />
    ) : (
      <TimelineEvent key={event.id} event={event} selectedMetrics={selectedMetrics} availableMetrics={availableMetrics} />
    );

  return (
    <div className="flex flex-col space-y-8 relative">
      <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-gray-200" />
      {groups.map((group, idx) => {
        if (group.length === 1) return renderEvent(group[0]);

        // Concurrent group — events are newest-first, so group[0] is latest.
        const latest = new Date(group[0].timestamp);
        const earliest = new Date(group[group.length - 1].timestamp);
        const services = [...new Set(group.flatMap(e => e.affected_services))];
        const serviceLabel = services.length <= 3 ? services.join(', ') : `${services.slice(0, 3).join(', ')} +${services.length - 3} more`;

        return (
          <div key={`cg-${idx}`} className="relative">
            {/* Amber line overlay replaces gray line for the duration of this group */}
            <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-amber-300 z-10" />

            {/* Group header */}
            <div className="ml-12 mb-4 flex flex-wrap items-center gap-2">
              <span className="flex items-center gap-1.5 px-3 py-1 bg-amber-50 border border-amber-200 rounded-full text-xs font-bold text-amber-700 shadow-sm">
                <Layers className="w-3 h-3" />
                {group.length} concurrent deployments
              </span>
              <span className="text-xs text-gray-500 font-medium">{serviceLabel}</span>
              <span className="text-xs text-gray-400 font-mono">{fmt(earliest)}–{fmt(latest)}</span>
            </div>

            {/* Events */}
            <div className="flex flex-col space-y-8">
              {group.map(renderEvent)}
            </div>
          </div>
        );
      })}
    </div>
  );
}
