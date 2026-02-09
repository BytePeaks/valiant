
import { useState, useEffect } from 'react';
import { Clock, Zap, GitBranch, Box, Bot, Info, Hourglass, Loader2, Tag, ShieldCheck, Settings2, Activity, ExternalLink } from 'lucide-react';
import { timeAgo, getImpactColor } from '../../lib/utils';
import type { ChangeEvent, ImpactAnalysis, TimelineEventProps } from '../../lib/api';
import type { MetricInfo } from '../promql-modal';
import { fetchAvailableMetrics, analyzeImpact, fetchServicePreferences, saveServicePreferences } from '../../lib/api';
import { MetricDelta } from './metric-delta';
import { METRIC_CONFIG, CORE_METRICS } from './constants';
import { getIcon } from '../icons';

export default function TimelineEvent({ event }: TimelineEventProps) {
  const [analysis, setAnalysis] = useState<ImpactAnalysis | null>(null);
  const [loading, setLoading] = useState(false);
  const [isMetricSelectionModalOpen, setIsMetricSelectionModalOpen] = useState(false);
  const [availableMetrics, setAvailableMetrics] = useState<MetricInfo[]>([]);
  const [selectedMetrics, setSelectedMetrics] = useState<Set<string>>(new Set(CORE_METRICS));
  const [selectedService, setSelectedService] = useState<string | null>(null);

  const isPending = event.analysis_status === 'pending';
  const isCompleted = event.analysis_status === 'completed';

  useEffect(() => {
    if (event.affected_services.length > 0) {
      setSelectedService(event.affected_services[0]);
    }
  }, [event.affected_services]);

  useEffect(() => {
    if (!selectedService) return;

    Promise.all([fetchAvailableMetrics(), fetchServicePreferences(selectedService)]).then(
      ([available, preferences]) => {
        setAvailableMetrics(available);
        if (preferences.length > 0) {
          setSelectedMetrics(new Set(preferences));
        } else {
          // Default to core metrics if no preferences are saved
          const coreMetrics = available.filter(m => CORE_METRICS.includes(m.name)).map(m => m.name);
          setSelectedMetrics(new Set(coreMetrics));
        }
      }
    );
  }, [selectedService]);

  const handleAnalyze = async () => {
    setLoading(true);
    try {
      const result = await analyzeImpact(event.id);
      setAnalysis(result);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleCustomizeClick = () => {
    setIsMetricSelectionModalOpen(true);
  };

  const handleMetricSelectionChange = (metric: string, isSelected: boolean) => {
    const newSelected = new Set(selectedMetrics);
    if (isSelected) {
      newSelected.add(metric);
    } else {
      newSelected.delete(metric);
    }
    setSelectedMetrics(newSelected);
  };

  const handleSavePreferences = async () => {
    if (!selectedService) return;

    await saveServicePreferences(selectedService, Array.from(selectedMetrics));
    setIsMetricSelectionModalOpen(false);
  };

  const getSourceIcon = () => {
    const type = event.trigger_type || event.source;
    switch (type?.toLowerCase()) {
      case 'gitops':
        return <GitBranch className="w-4 h-4" />;
      case 'kubernetes':
        return <Box className="w-4 h-4" />;
      case 'ci':
      case 'ci-cd':
        return <Bot className="w-4 h-4" />;
      default:
        return <Info className="w-4 h-4" />;
    }
  };

  const metricsToRender = Array.from(selectedMetrics).sort((a, b) => {
    const indexA = CORE_METRICS.indexOf(a);
    const indexB = CORE_METRICS.indexOf(b);
    if (indexA !== -1 && indexB !== -1) return indexA - indexB;
    if (indexA !== -1) return -1;
    if (indexB !== -1) return 1;
    return a.localeCompare(b);
  });

  return (
    <div className="ml-12 p-6 bg-white rounded-2xl shadow-sm border border-gray-100 transition-all hover:shadow-md hover:border-blue-100 group">
      {/* Event Header */}
      <div className="flex justify-between items-start">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-[10px] font-bold text-blue-600 uppercase tracking-widest">
            {getSourceIcon()}
            {event.trigger_type || event.source}
          </div>
          <h3 className="text-xl font-bold text-gray-900 group-hover:text-blue-700 transition-colors">{event.summary}</h3>
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Clock className="w-3 h-3" />
            {new Date(event.timestamp).toLocaleString()}
            <span className="flex items-center gap-1 px-2 py-0.5 bg-blue-600 text-white text-[10px] font-bold rounded-full shadow-sm">
              <Clock className="w-2 h-2" />
              {timeAgo(new Date(event.timestamp))}
            </span>
            {event.affected_services?.map(service => (
              <span key={service} className="flex items-center gap-1 px-2 py-0.5 bg-emerald-50 text-emerald-600 text-[10px] font-bold rounded-full border border-emerald-100">
                <Tag className="w-2 h-2" />
                {service}
              </span>
            ))}
          </div>
        </div>

        {/* Actions Area */}
        <div className="flex items-center space-x-2">
          {!analysis && !loading && isPending && (
            <div className="flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-500 text-sm font-bold rounded-xl border border-gray-200 cursor-not-allowed opacity-75" title="Data collection in progress">
              <Hourglass className="w-4 h-4 animate-pulse" />
              Pending
            </div>
          )}

          {!analysis && !loading && !isPending && !isCompleted && (
            <button
              onClick={handleAnalyze}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white text-sm font-bold rounded-xl hover:bg-blue-700 transition-all shadow-lg shadow-blue-200"
            >
              <Zap className="w-4 h-4 fill-current" />
              Analyze
            </button>
          )}

          {!analysis && !loading && isCompleted && (
            <button
              onClick={handleAnalyze}
              className="flex items-center gap-2 px-4 py-2 bg-emerald-600 text-white text-sm font-bold rounded-xl hover:bg-emerald-700 transition-all shadow-lg shadow-emerald-200"
            >
              <ShieldCheck className="w-4 h-4" />
              Analyzed
            </button>
          )}

          {loading && (
            <div className="flex items-center gap-2 px-4 py-2 text-blue-600 font-bold text-sm">
              <Loader2 className="w-4 h-4 animate-spin" />
              Analyzing...
            </div>
          )}
        </div>
      </div>

      {/* Analysis Section */}
      {analysis && (
        <div className="mt-6 pt-6 border-t border-gray-100 animate-in fade-in slide-in-from-top-2 duration-300">
          <div className="flex items-center p-4 bg-gray-50 rounded-xl mb-6 border border-gray-100">
            <div className="flex-1 text-center border-r border-gray-200">
              <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Impact Score</span>
              <div className={`text-2xl font-black mt-1 ${getImpactColor(analysis.impact_level)}`}>
                {(analysis.impact_score * 100).toFixed(0)}
                <span className="text-xs text-gray-400 font-normal">%</span>
              </div>
            </div>
            <div className="flex-1 text-center border-r border-gray-200">
              <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Severity</span>
              <div className={`text-sm font-bold mt-2 px-3 py-0.5 inline-block rounded-full bg-white border border-gray-200 shadow-sm ${getImpactColor(analysis.impact_level)}`}>
                {analysis.impact_level}
              </div>
            </div>
            <div className="flex-1 text-center">
              <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Confidence</span>
              <div className={`text-xl font-black mt-1 flex items-center justify-center gap-1 ${analysis.confidence_score > 0.7 ? 'text-emerald-600' : 'text-orange-500'}`}>
                <ShieldCheck className="w-3 h-3" />
                {(analysis.confidence_score * 100).toFixed(0)}
                <span className="text-xs text-gray-400 font-normal">%</span>
              </div>
            </div>
          </div>
          <div>
            {event.affected_services.length > 1 && (
              <div className="mb-4 flex items-center flex-wrap">
                <span className="text-sm font-bold text-gray-500 mr-2">Service:</span>
                <div className="flex flex-wrap gap-2">
                  {event.affected_services.map(service => (
                    <button
                      key={service}
                      onClick={() => setSelectedService(service)}
                      className={`flex items-center gap-1 px-2 py-0.5 text-xs font-bold rounded-md ${selectedService === service ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-700'}`}
                    >
                      <Tag className="w-2 h-2" />
                      {service}
                    </button>
                  ))}
                </div>
              </div>
            )}
            <div className="flex justify-between items-center mb-3">
              <h4 className="text-xs font-bold text-gray-400 uppercase tracking-widest flex items-center gap-2">
                <Activity className="w-3 h-3" />
                Metric Shifts (vs Baseline)
              </h4>
              <div className="relative group/button">
                <button
                  onClick={handleCustomizeClick}
                  className="flex items-center gap-2 px-3 py-1 bg-gray-100 text-gray-600 text-xs font-bold rounded-lg hover:bg-gray-200 transition-all border border-gray-200"
                >
                  <Settings2 className="w-3 h-3" />
                  Customize
                </button>
                <div className="absolute bottom-full mb-2 hidden group-hover/button:block w-48 p-2 bg-gray-900 text-white text-[10px] rounded shadow-lg z-10 text-center pointer-events-none">
                  Choose which metrics are displayed. These preferences are saved for this service.
                  <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
                </div>
              </div>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3 mb-6">
              {metricsToRender.map(metricName => {
                const metricInfo = availableMetrics.find(m => m.name === metricName);
                const allDeltas: Record<string, number> = {
                  error_rate: analysis.deltas.error_rate,
                  latency_p95_ms: analysis.deltas.latency_p95_ms,
                  rps: analysis.deltas.rps,
                  cpu: analysis.deltas.cpu,
                  memory: analysis.deltas.memory,
                  ...analysis.deltas.additional_metrics,
                };
                const deltaValue = allDeltas[metricName] || 0;
                const config = METRIC_CONFIG[metricName] || { label: metricName, description: `Custom metric: ${metricName}` };

                return (
                  <MetricDelta
                    key={metricName}
                    label={config.label}
                    delta={deltaValue}
                    icon={getIcon(metricInfo?.icon)}
                    description={config.description}
                    reverse={config.reverse}
                  />
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* Contextual Links Section */}
      {((analysis?.change_event?.contextual_links?.length ?? 0) > 0 || (event.contextual_links?.length ?? 0) > 0) && (
        <div className="mt-6 pt-6 border-t border-gray-100 animate-in fade-in slide-in-from-top-2 duration-300">
          <h4 className="text-xs font-bold text-gray-400 uppercase tracking-widest mb-3">
            Contextual Links
          </h4>
          <div className="flex flex-wrap gap-2">
            {((analysis && analysis.change_event.contextual_links) || event.contextual_links)?.map((link, idx) => (
              <a
                key={idx}
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-3 py-1 bg-blue-100 text-blue-800 text-xs font-semibold rounded-full hover:bg-blue-200 transition-colors shadow-sm"
              >
                <ExternalLink className="w-3 h-3" />
                {link.name}
              </a>
            ))}
          </div>
        </div>
      )}

      {/* Metric Selection Modal */}
      {isMetricSelectionModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex justify-center items-center z-50">
          <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md">
            <h3 className="text-lg font-bold mb-4">Customize Visible Metrics</h3>
            <div className="space-y-2">
              {availableMetrics.map(metric => (
                <label key={metric.name} className="flex items-center space-x-3">
                  <input
                    type="checkbox"
                    className="form-checkbox h-5 w-5 text-blue-600"
                    checked={selectedMetrics.has(metric.name)}
                    onChange={e => handleMetricSelectionChange(metric.name, e.target.checked)}
                  />
                  <span className="text-gray-700">{metric.name}</span>
                </label>
              ))}
            </div>
            <div className="mt-6 flex justify-end space-x-2">
              <button
                onClick={() => setIsMetricSelectionModalOpen(false)}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
              >
                Cancel
              </button>
              <button
                onClick={handleSavePreferences}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700"
              >
                Save Preferences
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}