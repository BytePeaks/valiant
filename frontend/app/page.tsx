'use client';

import { useEffect, useState } from 'react';
import { ChangeEvent, fetchChangeEvents } from '@/lib/api';
import Timeline from '@/components/timeline/timeline';

export default function Home() {
  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchChangeEvents()
      .then(setEvents)
      .finally(() => setLoading(false));
  }, []);

  return (
    <main className="min-h-screen bg-gray-50 p-8 md:p-24">
      <div className="max-w-4xl mx-auto">
        <header className="mb-12">
          <h1 className="text-4xl font-black text-gray-900 tracking-tight">VALIANT</h1>
          <p className="text-gray-500">Change Impact Radar</p>
        </header>
        
        <section>
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-xl font-bold text-gray-800">Recent Changes</h2>
            <button 
              onClick={() => fetchChangeEvents().then(setEvents)}
              className="text-sm text-blue-600 hover:underline"
            >
              Refresh
            </button>
          </div>

          {loading ? (
            <div className="text-center py-20 text-gray-400">Loading events...</div>
          ) : events.length > 0 ? (
            <Timeline events={events} />
          ) : (
            <div className="text-center py-20 bg-white rounded-lg border-2 border-dashed border-gray-200 text-gray-400">
              No change events found.
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
