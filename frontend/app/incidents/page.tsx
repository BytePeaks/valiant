'use client';

import { useState, useCallback } from 'react';
import { RankedChange, RankingsResponse, fetchRankings, fetchServices } from '@/lib/api';
import { useEffect } from 'react';
import { getImpactColor } from '@/lib/utils';
import {
  AlertTriangle,
  ArrowLeft,
  Clock,
  Search,
  ShieldCheck,
  Target,
  Loader2,
  TrendingUp,
} from 'lucide-react';
import Link from 'next/link';

function LikelihoodBar({ value, label }: { value: number; label: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest w-20 text-right shrink-0">
        {label}
      </span>
      <div className="flex-1 h-2 bg-gray-100 rounded-full overflow-hidden">
        <div
          className="h-full bg-blue-500 rounded-full transition-all"
          style={{ width: `${(value * 100).toFixed(0)}%` }}
        />
      </div>
      <span className="text-xs font-bold text-gray-600 w-10">
        {(value * 100).toFixed(0)}%
      </span>
    </div>
  );
}

export default function IncidentsPage() {
  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [rankings, setRankings] = useState<RankingsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchServices().then(setServices);
  }, []);

  // Default to last 2 hours when service is selected
  useEffect(() => {
    if (selectedService && !dateFrom && !dateTo) {
      const now = new Date();
      const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000);
      setDateFrom(twoHoursAgo.toISOString().slice(0, 16));
      setDateTo(now.toISOString().slice(0, 16));
    }
  }, [selectedService, dateFrom, dateTo]);

  const handleSearch = useCallback(async () => {
    if (!selectedService || !dateFrom || !dateTo) {
      setError('Please select a service and time range');
      return;
    }

    setError('');
    setLoading(true);
    try {
      const result = await fetchRankings(
        selectedService,
        new Date(dateFrom).toISOString(),
        new Date(dateTo).toISOString()
      );
      setRankings(result);
    } catch (err) {
      setError('Failed to fetch rankings. Ensure the backend is running.');
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [selectedService, dateFrom, dateTo]);

  const topSuspect = rankings?.ranked?.[0] ?? null;

  return (
    <main className="min-h-screen bg-gray-50 p-8 md:p-24">
      <div className="max-w-4xl mx-auto">
        <header className="mb-8">
          <Link
            href="/"
            className="flex items-center gap-1 text-sm text-gray-400 hover:text-blue-600 font-medium mb-4 transition-colors"
          >
            <ArrowLeft className="w-3 h-3" />
            Dashboard
          </Link>
          <h1 className="text-3xl font-black text-gray-900 tracking-tight">
            Incident Investigation
          </h1>
          <p className="text-gray-500 font-medium text-sm mt-1">
            Identify which recent change most likely caused service degradation
          </p>
        </header>

        {/* Query Form */}
        <section className="bg-white rounded-2xl shadow-sm border border-gray-100 p-6 mb-8">
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
            <div>
              <label className="block text-xs font-bold text-gray-400 uppercase tracking-widest mb-2">
                Service
              </label>
              <select
                value={selectedService}
                onChange={(e) => setSelectedService(e.target.value)}
                className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm text-gray-700 focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300"
              >
                <option value="">Select service...</option>
                {services.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-bold text-gray-400 uppercase tracking-widest mb-2">
                From
              </label>
              <input
                type="datetime-local"
                value={dateFrom}
                onChange={(e) => setDateFrom(e.target.value)}
                className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm text-gray-700 focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300"
              />
            </div>
            <div>
              <label className="block text-xs font-bold text-gray-400 uppercase tracking-widest mb-2">
                To
              </label>
              <input
                type="datetime-local"
                value={dateTo}
                onChange={(e) => setDateTo(e.target.value)}
                className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm text-gray-700 focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300"
              />
            </div>
            <button
              onClick={handleSearch}
              disabled={loading || !selectedService}
              className="flex items-center justify-center gap-2 px-4 py-2 bg-blue-600 text-white text-sm font-bold rounded-xl hover:bg-blue-700 transition-all shadow-lg shadow-blue-200 disabled:opacity-50"
            >
              {loading ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Search className="w-4 h-4" />
              )}
              Investigate
            </button>
          </div>
          {error && (
            <p className="mt-3 text-sm text-red-500 font-medium">{error}</p>
          )}
        </section>

        {/* Results */}
        {rankings && (
          <>
            {/* Suspected Cause */}
            {topSuspect && (
              <section className="bg-white rounded-2xl shadow-sm border border-gray-100 p-6 mb-6">
                <div className="flex items-center gap-2 mb-4 text-xs font-bold text-gray-400 uppercase tracking-widest">
                  <Target className="w-3 h-3" />
                  Suspected Root Cause
                </div>
                <div className="flex items-start gap-4">
                  <div className="shrink-0 w-12 h-12 bg-red-50 rounded-xl flex items-center justify-center">
                    <AlertTriangle className="w-6 h-6 text-red-500" />
                  </div>
                  <div className="flex-1">
                    <h3 className="text-lg font-bold text-gray-900">
                      {topSuspect.analysis.change_event.summary}
                    </h3>
                    <div className="flex items-center gap-3 mt-1 text-sm text-gray-500">
                      <span className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {new Date(topSuspect.analysis.change_event.timestamp).toLocaleString()}
                      </span>
                      <span
                        className={`px-2 py-0.5 rounded-full text-xs font-bold ${getImpactColor(topSuspect.analysis.impact_level)}`}
                      >
                        {topSuspect.analysis.impact_level}
                      </span>
                    </div>

                    {/* Breakdown */}
                    <div className="mt-4 space-y-2">
                      <LikelihoodBar
                        value={topSuspect.likelihood_score}
                        label="Overall"
                      />
                      <LikelihoodBar
                        value={topSuspect.analysis.impact_score}
                        label="Impact"
                      />
                      <LikelihoodBar
                        value={topSuspect.temporal_proximity}
                        label="Temporal"
                      />
                      <LikelihoodBar
                        value={topSuspect.change_type_weight}
                        label="Risk"
                      />
                      <LikelihoodBar
                        value={topSuspect.service_scope}
                        label="Scope"
                      />
                    </div>

                    {/* Confidence */}
                    <div className="mt-4 flex items-center gap-2">
                      <ShieldCheck className="w-4 h-4 text-emerald-500" />
                      <span className="text-sm font-bold text-gray-600">
                        Confidence: {(topSuspect.analysis.confidence_score * 100).toFixed(0)}%
                      </span>
                    </div>
                  </div>
                </div>
              </section>
            )}

            {/* All Ranked Changes */}
            <section className="bg-white rounded-2xl shadow-sm border border-gray-100 p-6">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2 text-xs font-bold text-gray-400 uppercase tracking-widest">
                  <TrendingUp className="w-3 h-3" />
                  All Changes Ranked by Likelihood
                </div>
                <span className="text-xs text-gray-400 font-medium">
                  {rankings.total} change{rankings.total !== 1 ? 's' : ''} found
                </span>
              </div>

              {rankings.ranked.length === 0 ? (
                <div className="text-center py-12 text-gray-400">
                  No changes found in this time window.
                </div>
              ) : (
                <div className="space-y-3">
                  {rankings.ranked.map((rc: RankedChange) => (
                    <div
                      key={rc.analysis.change_event.id}
                      className={`p-4 rounded-xl border transition-all ${
                        rc.rank === 1
                          ? 'border-red-200 bg-red-50/50'
                          : 'border-gray-100 hover:border-blue-100'
                      }`}
                    >
                      <div className="flex items-center gap-4">
                        {/* Rank badge */}
                        <div
                          className={`shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-sm font-black ${
                            rc.rank === 1
                              ? 'bg-red-500 text-white'
                              : rc.rank <= 3
                              ? 'bg-orange-100 text-orange-600'
                              : 'bg-gray-100 text-gray-500'
                          }`}
                        >
                          {rc.rank}
                        </div>

                        {/* Event info */}
                        <div className="flex-1 min-w-0">
                          <div className="font-bold text-gray-900 truncate">
                            {rc.analysis.change_event.summary}
                          </div>
                          <div className="flex items-center gap-3 mt-0.5 text-xs text-gray-400">
                            <span className="flex items-center gap-1">
                              <Clock className="w-3 h-3" />
                              {new Date(rc.analysis.change_event.timestamp).toLocaleString()}
                            </span>
                            <span className="px-1.5 py-0.5 bg-gray-100 rounded text-gray-500 font-medium">
                              {rc.analysis.change_event.change_type}
                            </span>
                          </div>
                        </div>

                        {/* Scores */}
                        <div className="shrink-0 flex items-center gap-4 text-right">
                          <div>
                            <div className="text-[10px] font-bold text-gray-400 uppercase">
                              Likelihood
                            </div>
                            <div className="text-lg font-black text-gray-800">
                              {(rc.likelihood_score * 100).toFixed(0)}%
                            </div>
                          </div>
                          <div>
                            <div className="text-[10px] font-bold text-gray-400 uppercase">
                              Impact
                            </div>
                            <div
                              className={`text-lg font-black ${getImpactColor(rc.analysis.impact_level)}`}
                            >
                              {rc.analysis.impact_level}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </div>
    </main>
  );
}
