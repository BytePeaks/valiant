'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import {
  ChangeEvent, EventsResponse, fetchChangeEvents,
  fetchAvailableMetrics, fetchServicePreferences, saveServicePreferences,
} from '@/lib/api';
import type { MetricInfo } from '@/components/promql-modal';
import Timeline from '@/components/timeline/timeline';
import { getIcon } from '@/components/icons';
import { METRIC_CONFIG, CORE_METRICS } from '@/components/timeline/constants';
import { ChevronLeft, Activity, Filter, RefreshCcw, BarChart2, Check } from 'lucide-react';
import Link from 'next/link';

export default function ServicePage() {
  const { serviceName } = useParams();
  const router = useRouter();
  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [availableMetrics, setAvailableMetrics] = useState<MetricInfo[]>([]);
  const [selectedMetrics, setSelectedMetrics] = useState<Set<string>>(new Set(CORE_METRICS));
  const [savingPrefs, setSavingPrefs] = useState(false);
  const [prefsSaved, setPrefsSaved] = useState(false);

  const fetchServiceEvents = () => {
    setLoading(true);
    const decodedName = decodeURIComponent(serviceName as string);
    fetchChangeEvents({ service: decodedName, limit: 50 })
      .then((response: EventsResponse) => {
        setEvents(response.events);
        setTotal(response.total);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchServiceEvents();
  }, [serviceName]);

  useEffect(() => {
    const decodedName = decodeURIComponent(serviceName as string);
    Promise.all([fetchAvailableMetrics(), fetchServicePreferences(decodedName)]).then(
      ([available, prefs]) => {
        setAvailableMetrics(available);
        if (prefs.length > 0) {
          setSelectedMetrics(new Set(prefs));
        } else {
          setSelectedMetrics(new Set(available.filter(m => CORE_METRICS.includes(m.name)).map(m => m.name)));
        }
      }
    );
  }, [serviceName]);

  const handleToggleMetric = (name: string) => {
    setSelectedMetrics(prev => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      return next;
    });
    setPrefsSaved(false);
  };

  const handleSavePreferences = async () => {
    setSavingPrefs(true);
    const decodedName = decodeURIComponent(serviceName as string);
    await saveServicePreferences(decodedName, Array.from(selectedMetrics));
    setSavingPrefs(false);
    setPrefsSaved(true);
    setTimeout(() => setPrefsSaved(false), 2000);
  };

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
                <div className="text-2xl font-black text-gray-900">{total}</div>
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

        {availableMetrics.length > 0 && (
          <section className="mb-8 bg-white p-5 rounded-2xl border border-gray-100 shadow-sm">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2 text-xs font-bold text-gray-400 uppercase tracking-widest">
                <BarChart2 className="w-3 h-3" />
                Metric Filters
              </div>
              <button
                onClick={handleSavePreferences}
                disabled={savingPrefs}
                className={`flex items-center gap-1.5 px-3 py-1 text-xs font-bold rounded-lg transition-all border ${
                  prefsSaved
                    ? 'bg-emerald-50 text-emerald-600 border-emerald-200'
                    : 'bg-gray-50 text-gray-600 border-gray-200 hover:bg-gray-100'
                }`}
              >
                <Check className="w-3 h-3" />
                {prefsSaved ? 'Saved!' : 'Save Preferences'}
              </button>
            </div>
            <div className="flex flex-wrap gap-2">
              {availableMetrics.map(metric => {
                const isSelected = selectedMetrics.has(metric.name);
                const config = METRIC_CONFIG[metric.name];
                return (
                  <button
                    key={metric.name}
                    onClick={() => handleToggleMetric(metric.name)}
                    className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-bold border transition-all ${
                      isSelected
                        ? 'bg-blue-600 text-white border-blue-600 shadow-sm shadow-blue-200'
                        : 'bg-gray-50 text-gray-500 border-gray-200 hover:bg-gray-100'
                    }`}
                  >
                    {getIcon(metric.icon)}
                    {config?.label || metric.name}
                  </button>
                );
              })}
            </div>
            <p className="text-[10px] text-gray-400 mt-2">
              {selectedMetrics.size} of {availableMetrics.length} metrics shown in analysis cards below
            </p>
          </section>
        )}

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
            <Timeline events={events} selectedMetrics={selectedMetrics} availableMetrics={availableMetrics} />
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
