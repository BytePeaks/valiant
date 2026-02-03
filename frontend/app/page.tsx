'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { ChangeEvent, EventsResponse, fetchChangeEvents, fetchServices, fetchNamespaces } from '@/lib/api';
import Timeline from '@/components/timeline/timeline';
import { RefreshCcw, Filter, ExternalLink, ChevronDown, ChevronUp, Layers, Search, X } from 'lucide-react';
import Link from 'next/link';

const PAGE_SIZE = 10;

export default function Home() {
  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState<string | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedNamespace, setSelectedNamespace] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [filtersVisible, setFiltersVisible] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const abortControllerRef = useRef<AbortController | null>(null);

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  const fetchEvents = useCallback((currentOffset: number, service: string | null, namespace: string | null, search: string) => {
    // Cancel any in-flight request
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    const controller = new AbortController();
    abortControllerRef.current = controller;

    setLoading(true);
    fetchChangeEvents(
      {
        limit: PAGE_SIZE,
        offset: currentOffset,
        ...(service ? { service } : {}),
        ...(namespace ? { namespace } : {}),
        ...(search ? { search } : {}),
      },
      controller.signal
    )
      .then((response: EventsResponse) => {
        if (currentOffset === 0) {
          setEvents(response.events);
        } else {
          setEvents((prev) => [...prev, ...response.events]);
        }
        setTotal(response.total);
        setOffset(currentOffset);
      })
      .catch((err) => {
        if (err instanceof DOMException && err.name === 'AbortError') return;
        throw err;
      })
      .finally(() => setLoading(false));
  }, []);

  // Load services and namespaces once on mount
  useEffect(() => {
    Promise.all([fetchServices(), fetchNamespaces()])
      .then(([servicesData, namespacesData]) => {
        setServices(servicesData);
        setNamespaces(namespacesData);
      });
  }, []);

  // Re-fetch when filters or search change
  useEffect(() => {
    fetchEvents(0, selectedService, selectedNamespace, debouncedSearch);
  }, [selectedService, selectedNamespace, debouncedSearch, fetchEvents]);

  const handleShowMore = () => {
    const nextOffset = offset + PAGE_SIZE;
    fetchEvents(nextOffset, selectedService, selectedNamespace, debouncedSearch);
  };

  const handleRefresh = () => {
    fetchEvents(0, selectedService, selectedNamespace, debouncedSearch);
  };

  const hasMore = events.length < total;

  return (
    <main className="min-h-screen bg-gray-50 p-8 md:p-24">
      <div className="max-w-4xl mx-auto">
        <header className="mb-12">
          <h1 className="text-4xl font-black text-gray-900 tracking-tight italic uppercase">Valiant<span className="text-blue-600">.</span></h1>
          <p className="text-gray-500 font-medium">Change Impact Radar for Teams</p>
        </header>


        <section className="mb-12">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2 text-sm font-bold text-gray-400 uppercase tracking-widest">
              <Filter className="w-3 h-3" />
              <span>Filters</span>
            </div>
            <button onClick={() => setFiltersVisible(!filtersVisible)} className="p-1 rounded-full hover:bg-gray-200">
              {filtersVisible ? <ChevronUp className="w-4 h-4 text-gray-500" /> : <ChevronDown className="w-4 h-4 text-gray-500" />}
            </button>
          </div>

          {filtersVisible && (
            <div>
              {/* Search Events */}
              <section className="mb-8">
                <div className="flex items-center gap-2 mb-4 text-xs font-bold text-gray-400 uppercase tracking-widest">
                  Search Events
                </div>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search by commit SHA, summary, image tag..."
                    className="w-full pl-10 pr-10 py-2 rounded-full border border-gray-200 shadow-sm text-sm text-gray-700 placeholder-gray-400 focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300 transition-all"
                  />
                  {searchQuery && (
                    <button
                      onClick={() => setSearchQuery('')}
                      className="absolute right-3 top-1/2 -translate-y-1/2 p-0.5 rounded-full text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  )}
                </div>
              </section>

              {/* Namespace Filter */}
              <section className="mb-8">
                <div className="flex items-center gap-2 mb-4 text-xs font-bold text-gray-400 uppercase tracking-widest">
                  Filter by Namespace
                </div>
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => setSelectedNamespace(null)}
                    className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all border ${!selectedNamespace ? 'bg-blue-600 border-blue-600 text-white' : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'}`}
                  >
                    All Namespaces
                  </button>
                  {namespaces.map(namespace => (
                    <button
                      key={namespace}
                      onClick={() => setSelectedNamespace(namespace === selectedNamespace ? null : namespace)}
                      className={`flex items-center gap-1 px-4 py-1.5 rounded-full text-xs font-bold transition-all border ${selectedNamespace === namespace ? 'bg-blue-600 border-blue-600 text-white' : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'}`}
                    >
                      <Layers className="w-3 h-3" />
                      {namespace}
                    </button>
                  ))}
                </div>
              </section>

              {/* Service Filter */}
              <section>
                <div className="flex items-center gap-2 mb-4 text-xs font-bold text-gray-400 uppercase tracking-widest">
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
            </div>
          )}
        </section>


        <section>
          <div className="flex justify-between items-center mb-8 bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
            <div>
              <h2 className="text-xl font-bold text-gray-800">
                {selectedService ? `${selectedService} Changes` : 'Recent Changes'}
              </h2>
              <p className="text-xs text-gray-400 font-medium mt-0.5">
                Showing {events.length} of {total} events
              </p>
            </div>
            <button
              onClick={handleRefresh}
              disabled={loading}
              className="flex items-center gap-2 px-4 py-2 bg-gray-50 hover:bg-gray-100 text-gray-700 rounded-lg text-sm font-semibold transition-all border border-gray-200"
            >
              <RefreshCcw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>

          {loading && events.length > 0 && (
            <div className="flex items-center gap-2 text-xs text-gray-400 font-medium mb-4">
              <RefreshCcw className="w-3 h-3 animate-spin" />
              Updating results...
            </div>
          )}

          {loading && events.length === 0 ? (
            <div className="text-center py-20 text-gray-400">Loading events...</div>
          ) : events.length > 0 ? (
            <>
              <Timeline events={events} />

              {hasMore && (
                <div className="mt-8 text-center">
                  <button
                    onClick={handleShowMore}
                    disabled={loading}
                    className="px-6 py-2 bg-white border border-gray-300 rounded-full text-sm font-semibold text-gray-700 hover:bg-gray-50 hover:border-gray-400 transition-all shadow-sm disabled:opacity-50"
                  >
                    Show More ({total - events.length} remaining)
                  </button>
                </div>
              )}
            </>
          ) : (
            <div className="text-center py-20 bg-white rounded-lg border-2 border-dashed border-gray-200 text-gray-400">
              {debouncedSearch
                ? `No events found matching "${debouncedSearch}"`
                : 'No change events found.'}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
