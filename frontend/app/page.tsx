'use client';

import { useEffect, useState } from 'react';
import { ChangeEvent, fetchChangeEvents, fetchServices } from '@/lib/api';
import Timeline from '@/components/timeline/timeline';
import { RefreshCcw, Filter, ExternalLink } from 'lucide-react';
import Link from 'next/link';

export default function Home() {
  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [visibleCount, setVisibleCount] = useState(5);

  const fetchData = () => {
    setLoading(true);
    Promise.all([fetchChangeEvents(), fetchServices()])
      .then(([eventsData, servicesData]) => {
        setEvents(eventsData);
        setServices(servicesData);
        setVisibleCount(5);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleShowMore = () => {
    setVisibleCount((prev) => prev + 5);
  };

  const filteredEvents = selectedService 
    ? events.filter(e => e.affected_services.includes(selectedService))
    : events;

  const visibleEvents = filteredEvents.slice(0, visibleCount);
  const hasMore = visibleCount < filteredEvents.length;

  return (
    <main className="min-h-screen bg-gray-50 p-8 md:p-24">
      <div className="max-w-4xl mx-auto">
        <header className="mb-12">
          <h1 className="text-4xl font-black text-gray-900 tracking-tight italic uppercase">Valiant<span className="text-blue-600">.</span></h1>
          <p className="text-gray-500 font-medium">Change Impact Radar for Teams</p>
        </header>

        {/* Service Filter */}
        <section className="mb-12">
          <div className="flex items-center gap-2 mb-4 text-sm font-bold text-gray-400 uppercase tracking-widest">
            <Filter className="w-3 h-3" />
            Filter by Service
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setSelectedService(null)}
              className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all border ${!selectedService ? 'bg-blue-600 border-blue-600 text-white' : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'}`}
            >
              All Services
            </button>
            {services.map(service => (
              <div 
                key={service} 
                className={`flex items-center gap-2 px-4 py-1.5 rounded-full text-xs font-bold transition-all border ${selectedService === service ? 'bg-blue-600 border-blue-600 text-white' : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'}`}
              >
                <button
                  onClick={() => setSelectedService(service === selectedService ? null : service)}
                  className="focus:outline-none"
                >
                  {service}
                </button>
                <Link 
                  href={`/services/${encodeURIComponent(service)}`}
                  className={`p-0.5 rounded-full transition-colors ${selectedService === service ? 'hover:bg-blue-500 text-blue-100' : 'hover:bg-gray-100 text-gray-400'}`}
                  title={`Go to ${service} dashboard`}
                >
                  <ExternalLink className="w-3 h-3" />
                </Link>
              </div>
            ))}
          </div>
        </section>
        
        <section>
          <div className="flex justify-between items-center mb-8 bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
            <h2 className="text-xl font-bold text-gray-800">
              {selectedService ? `${selectedService} Changes` : 'Recent Changes'}
            </h2>
            <button 
              onClick={fetchData}
              disabled={loading}
              className="flex items-center gap-2 px-4 py-2 bg-gray-50 hover:bg-gray-100 text-gray-700 rounded-lg text-sm font-semibold transition-all border border-gray-200"
            >
              <RefreshCcw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>

          {loading && events.length === 0 ? (
            <div className="text-center py-20 text-gray-400">Loading events...</div>
          ) : events.length > 0 ? (
            <>
              <Timeline events={visibleEvents} />
              
              {hasMore && (
                <div className="mt-8 text-center">
                  <button
                    onClick={handleShowMore}
                    className="px-6 py-2 bg-white border border-gray-300 rounded-full text-sm font-semibold text-gray-700 hover:bg-gray-50 hover:border-gray-400 transition-all shadow-sm"
                  >
                    Show More ({events.length - visibleCount} remaining)
                  </button>
                </div>
              )}
            </>
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
