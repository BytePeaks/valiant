'use client';

import { ChangeEvent } from '@/lib/api';
import TimelineEvent from './timeline-event';

interface TimelineProps {
  events: ChangeEvent[];
}

export default function Timeline({ events }: TimelineProps) {
  return (
    <div className="flex flex-col space-y-8 relative">
      <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-gray-200"></div>
      {events.map((event) => (
        <TimelineEvent key={event.id} event={event} />
      ))}
    </div>
  );
}
