'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { ChangeEvent, EventsResponse, fetchChangeEvents, fetchServices, fetchNamespaces, CustomMetricInfo, fetchCustomMetrics } from '@/lib/api'; // Added CustomMetricInfo, fetchCustomMetrics
import Timeline from '@/components/timeline/timeline';
import { RefreshCcw, Filter, ExternalLink, ChevronDown, ChevronUp, Layers, Search, X, Zap, Info } from 'lucide-react'; // Added Info
import Link from 'next/link';
import PromQLModal, { MetricInfo } from '@/components/promql-modal'; // Added PromQLModal, MetricInfo

const PAGE_SIZE = 10;

// Metric definitions for the PromQL Modal
const METRIC_DEFINITIONS = [
  {
    name: "Service Request Latency (P99)",
    description: "P99 latency of HTTP requests to a service. Replace 'your_service' with the actual service name.",
    promql: "histogram_quantile(0.99, sum by (le, service_name) (rate(http_request_duration_seconds_bucket{job='valiant', service_name='your_service'}[5m])))"
  },
  {
    name: "Service Error Rate",
    description: "Rate of HTTP errors (status 5xx) for a service. Replace 'your_service' with the actual service name.",
    promql: "sum by (service_name) (rate(http_requests_total{job='valiant', service_name='your_service', status_code=~'5xx'}[5m])) / sum by (service_name) (rate(http_requests_total{job='valiant', service_name='your_service'}[5m]))"
  },
  {
    name: "Node CPU Utilization",
    description: "Average CPU utilization across all cores on a node.",
    promql: "1 - avg by (instance) (rate(node_cpu_seconds_total{mode='idle'}[5m]))"
  },
  {
    name: "Node Memory Usage",
    description: "Percentage of memory used on a node.",
    promql: "(node_memory_MemTotal_bytes - node_memory_MemFree_bytes) / node_memory_MemTotal_bytes * 100"
  }
];

const CHANGE_TYPES = [
  { value: 'deployment_rollout', label: 'Deployment' },
  { value: 'configmap_update', label: 'ConfigMap' },
  { value: 'secret_update', label: 'Secret' },
  { value: 'build_success', label: 'CI Build' },
  { value: 'tag_created', label: 'Tag' },
  { value: 'release_published', label: 'Release' },
  { value: 'branch_merged', label: 'Merge' },
];

export default function Home() {
  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState<string | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedNamespace, setSelectedNamespace] = useState<string | null>(null);
  const [selectedChangeType, setSelectedChangeType] = useState<string | null>(null);
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [loading, setLoading] = useState(true);
  const [filtersVisible, setFiltersVisible] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const abortControllerRef = useRef<AbortController | null>(null);
  const [isPromQLModalOpen, setIsPromQLModalOpen] = useState(false); // New state for modal
  const [dynamicCustomMetrics, setDynamicCustomMetrics] = useState<MetricInfo[]>([]); // New state for dynamic custom metrics
  const [loadingCustomMetrics, setLoadingCustomMetrics] = useState(true); // New state for loading custom metrics

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Fetch custom metrics on component mount
  useEffect(() => {
    const loadCustomMetrics = async () => {
      try {
        setLoadingCustomMetrics(true);
        const data = await fetchCustomMetrics();
        // Enrich data with a description for the modal, as it's not provided by the backend
        const enrichedData = data.map(metric => ({
            ...metric,
            description: `Custom metric: ${metric.name}`
        }));
        setDynamicCustomMetrics(enrichedData);
      } catch (error) {
        console.error("Failed to fetch custom metrics:", error);
        setDynamicCustomMetrics([]); // Ensure it's an empty array on error
      } finally {
        setLoadingCustomMetrics(false);
      }
    };
    loadCustomMetrics();
  }, []);

  const loadEvents = useCallback((
    currentOffset: number,
    service: string | null,
    namespace: string | null,
    search: string,
    changeType: string | null,
    from: string,
    to: string,
  ) => {
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
        ...(changeType ? { change_type: changeType } : {}),
        ...(from ? { from: new Date(from).toISOString() } : {}),
        ...(to ? { to: new Date(to).toISOString() } : {}),
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
    loadEvents(0, selectedService, selectedNamespace, debouncedSearch, selectedChangeType, dateFrom, dateTo);
  }, [selectedService, selectedNamespace, debouncedSearch, selectedChangeType, dateFrom, dateTo, loadEvents]);

  const handleShowMore = () => {
    const nextOffset = offset + PAGE_SIZE;
    loadEvents(nextOffset, selectedService, selectedNamespace, debouncedSearch, selectedChangeType, dateFrom, dateTo);
  };

  const handleRefresh = () => {
    loadEvents(0, selectedService, selectedNamespace, debouncedSearch, selectedChangeType, dateFrom, dateTo);
  };

  const hasMore = events.length < total;

  return (
    <main className="min-h-screen bg-gray-50 p-8 md:p-24">
      <div className="max-w-4xl mx-auto">
        <header className="mb-12 flex justify-between items-start">
          <div>
            <h1 className="text-4xl font-black text-gray-900 tracking-tight italic uppercase">Valiant<span className="text-blue-600">.</span></h1>
            <p className="text-gray-500 font-medium">Change Impact Radar for Teams</p>
          </div>
          <div className="flex items-center gap-4">
            <Link
              href="/incidents"
              className="flex items-center gap-2 px-4 py-2 bg-white border border-gray-200 text-gray-700 text-sm font-semibold rounded-xl hover:border-blue-300 hover:text-blue-600 transition-all shadow-sm"
            >
              <Zap className="w-4 h-4" />
              Investigate Incident
            </Link>
            <button
              onClick={() => setIsPromQLModalOpen(true)}
              className="w-8 h-8 rounded-full flex items-center justify-center bg-blue-100 text-blue-600 hover:bg-blue-200 transition-colors shrink-0"
              title="View Prometheus Metrics"
            >
              <Info className="w-4 h-4" />
            </button>
          </div>
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
              <section className="mb-8">
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

              {/* Date Range Filter */}
              <section className="mb-8">
                <div className="flex items-center gap-2 mb-4 text-xs font-bold text-gray-400 uppercase tracking-widest">
                  Date Range
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <input
                    type="datetime-local"
                    value={dateFrom}
                    onChange={(e) => setDateFrom(e.target.value)}
                    className="px-3 py-1.5 rounded-lg border border-gray-200 shadow-sm text-sm text-gray-700 focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300 transition-all"
                  />
                  <span className="text-xs text-gray-400 font-medium">to</span>
                  <input
                    type="datetime-local"
                    value={dateTo}
                    onChange={(e) => setDateTo(e.target.value)}
                    className="px-3 py-1.5 rounded-lg border border-gray-200 shadow-sm text-sm text-gray-700 focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300 transition-all"
                  />
                  {(dateFrom || dateTo) && (
                    <button
                      onClick={() => { setDateFrom(''); setDateTo(''); }}
                      className="px-3 py-1.5 rounded-full text-xs font-bold text-gray-500 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                    >
                      <X className="w-3 h-3 inline mr-1" />
                      Clear
                    </button>
                  )}
                </div>
              </section>

              {/* Change Type Filter */}
              <section>
                <div className="flex items-center gap-2 mb-4 text-xs font-bold text-gray-400 uppercase tracking-widest">
                  Filter by Change Type
                </div>
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => setSelectedChangeType(null)}
                    className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all border ${!selectedChangeType ? 'bg-blue-600 border-blue-600 text-white' : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'}`}
                  >
                    All Types
                  </button>
                  {CHANGE_TYPES.map(ct => (
                    <button
                      key={ct.value}
                      onClick={() => setSelectedChangeType(ct.value === selectedChangeType ? null : ct.value)}
                      className={`px-4 py-1.5 rounded-full text-xs font-bold transition-all border ${selectedChangeType === ct.value ? 'bg-blue-600 border-blue-600 text-white' : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'}`}
                    >
                      {ct.label}
                    </button>
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
      {/* PromQL Metrics Modal */}
              <PromQLModal
                  isOpen={isPromQLModalOpen}
                  onClose={() => setIsPromQLModalOpen(false)}
                  metrics={METRIC_DEFINITIONS}
                  customMetrics={dynamicCustomMetrics}
              />
    </main>
  );
}
