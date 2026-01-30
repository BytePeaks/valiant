'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { ChangeEvent, fetchChangeEvents } from '@/lib/api';
import Timeline from '@/components/timeline/timeline';
import { ChevronLeft, Activity, Filter, RefreshCcw } from 'lucide-react';
import Link from 'next/link';

export default function ServicePage() {
  const { serviceName } = useParams();
  const router = useRouter();
  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchServiceEvents = () => {
    setLoading(true);
    const decodedName = decodeURIComponent(serviceName as string);
    fetchChangeEvents()
      .then((data) => {
        // Filter for this specific service
        const filtered = data.filter(e => e.affected_services.includes(decodedName));
        setEvents(filtered);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchServiceEvents();
  }, [serviceName]);

  return (
    <main className="min-h-screen bg-gray-50 p-8 md:p-24">
      <div className="max-w-4xl mx-auto">
        <Link 
          href="/"
          className="inline-flex items-center gap-2 text-sm font-bold text-blue-600 hover:text-blue-800 transition-colors mb-8"
        >
          <ChevronLeft className="w-4 h-4" />
          Back to all services
        </Link>

        <header className="mb-12 bg-white p-8 rounded-3xl shadow-sm border border-gray-100 flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <div className="p-2 bg-blue-600 rounded-xl">
                <Activity className="w-6 h-6 text-white" />
              </div>
              <h1 className="text-3xl font-black text-gray-900 tracking-tight uppercase italic">
                {decodeURIComponent(serviceName as string)}
              </h1>
            </div>
            <p className="text-gray-500 font-medium">Service Analysis & Change History</p>
          </div>
          
          <div className="flex items-center gap-4">
             <div className="text-right hidden sm:block">
                <div className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Total Changes</div>
                <div className="text-2xl font-black text-gray-900">{events.length}</div>
             </div>
             <button 
              onClick={fetchServiceEvents}
              disabled={loading}
              className="flex items-center gap-2 px-4 py-2 bg-gray-50 hover:bg-gray-100 text-gray-700 rounded-xl text-sm font-bold transition-all border border-gray-200"
            >
              <RefreshCcw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>
        </header>

        <section>
          <div className="flex items-center gap-2 mb-6 text-sm font-bold text-gray-400 uppercase tracking-widest">
            <Filter className="w-3 h-3" />
            Filtered Change Timeline
          </div>

          {loading ? (
            <div className="space-y-4">
              {[1, 2, 3].map(i => (
                <div key={i} className="ml-12 h-32 bg-gray-200 animate-pulse rounded-2xl"></div>
              ))}
            </div>
          ) : events.length > 0 ? (
            <Timeline events={events} />
          ) : (
            <div className="text-center py-20 bg-white rounded-3xl border-2 border-dashed border-gray-200 text-gray-400 font-medium">
              No change events found for this service.
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
