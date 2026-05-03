'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import {
  Bot, Box, Zap, ShieldCheck, ArrowRight, GitBranch, Tag, Clock,
  Link2, Loader2, Settings2, Activity, ExternalLink, Hourglass,
  PackageCheck, ChevronDown, ChevronUp,
} from 'lucide-react';
import { timeAgo, getImpactColor } from '../../lib/utils';
import type { ChangeEvent, ImpactAnalysis } from '../../lib/api';
import type { MetricInfo } from '../promql-modal';
import { fetchAvailableMetrics, analyzeImpact, fetchServicePreferences, saveServicePreferences } from '../../lib/api';
import { MetricDelta } from './metric-delta';
import { METRIC_CONFIG, CORE_METRICS } from './constants';
import { getIcon } from '../icons';
import BlastRadiusDisplay from './blast-radius';
import { MetadataSection } from '../metadata-value';

interface Props {
  event: ChangeEvent;
  selectedMetrics?: Set<string>;
  availableMetrics?: MetricInfo[];
}

function shortSha(sha: string): string {
  return sha.substring(0, 7);
}

function shortImageTag(tag: string): string {
  const colon = tag.lastIndexOf(':');
  const label = colon !== -1 ? tag.slice(colon + 1) : tag;
  return label.length > 20 ? `…${label.slice(-18)}` : label;
}

function linkConfidenceLabel(type: string): string {
  switch (type) {
    case 'sha_match': return 'SHA Match';
    case 'image_tag_match': return 'Tag Match';
    case 'image_sha_inferred': return 'SHA Inferred';
    default: return type;
  }
}

function ImpactBadge({ level, score }: { level: string; score: number }) {
  const colors: Record<string, string> = {
    HIGH: 'bg-red-100 text-red-700 border-red-200',
    MEDIUM: 'bg-orange-100 text-orange-700 border-orange-200',
    LOW: 'bg-yellow-100 text-yellow-700 border-yellow-200',
    NONE: 'bg-emerald-100 text-emerald-700 border-emerald-200',
  };
  return (
    <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold border ${colors[level] ?? colors.NONE}`}>
      {level} {(score * 100).toFixed(0)}%
    </span>
  );
}

export default function DeploymentStoryCard({ event, selectedMetrics: pageSelectedMetrics, availableMetrics: pageAvailableMetrics }: Props) {
  const intent = event.linked_intent!;

  const [analysis, setAnalysis] = useState<ImpactAnalysis | null>(null);
  const [loading, setLoading] = useState(event.analysis_status === 'completed');
  const [detailsExpanded, setDetailsExpanded] = useState(false);
  const [storyExpanded, setStoryExpanded] = useState(false);
  const [isMetricModalOpen, setIsMetricModalOpen] = useState(false);
  const [availableMetrics, setAvailableMetrics] = useState<MetricInfo[]>(pageAvailableMetrics ?? []);
  const [selectedMetrics, setSelectedMetrics] = useState<Set<string>>(pageSelectedMetrics ?? new Set(CORE_METRICS));
  const [selectedService, setSelectedService] = useState<string | null>(
    event.affected_services[0] ?? null
  );

  const isPending = event.analysis_status === 'pending';
  const isCompleted = event.analysis_status === 'completed';

  const imageTag =
    event.metadata?.image_tag ||
    intent?.metadata?.image_tag ||
    null;

  const intentSha =
    intent?.metadata?.git_commit_sha ||
    intent?.metadata?.git_sha ||
    event.metadata?.git_commit_sha ||
    event.metadata?.git_sha ||
    null;

  useEffect(() => {
    if (pageSelectedMetrics) setSelectedMetrics(pageSelectedMetrics);
  }, [pageSelectedMetrics]);

  useEffect(() => {
    if (pageAvailableMetrics) setAvailableMetrics(pageAvailableMetrics);
  }, [pageAvailableMetrics]);

  useEffect(() => {
    if (!selectedService || pageSelectedMetrics) return;
    Promise.all([fetchAvailableMetrics(), fetchServicePreferences(selectedService)]).then(
      ([available, prefs]) => {
        setAvailableMetrics(available);
        if (prefs.length > 0) {
          setSelectedMetrics(new Set(prefs));
        } else {
          setSelectedMetrics(new Set(available.filter(m => CORE_METRICS.includes(m.name)).map(m => m.name)));
        }
      }
    );
  }, [selectedService]);

  useEffect(() => {
    if (isCompleted && !analysis) {
      setLoading(true);
      analyzeImpact(event.id)
        .then(result => { setAnalysis(result); setDetailsExpanded(false); })
        .catch(err => console.error(err))
        .finally(() => setLoading(false));
    }
  }, [isCompleted, event.id]);

  const handleAnalyze = async () => {
    setLoading(true);
    try {
      const result = await analyzeImpact(event.id);
      setAnalysis(result);
      setDetailsExpanded(true);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleSavePreferences = async () => {
    if (!selectedService) return;
    await saveServicePreferences(selectedService, Array.from(selectedMetrics));
    setIsMetricModalOpen(false);
  };

  const metricsToRender = Array.from(selectedMetrics).sort((a, b) => {
    const ia = CORE_METRICS.indexOf(a);
    const ib = CORE_METRICS.indexOf(b);
    if (ia !== -1 && ib !== -1) return ia - ib;
    if (ia !== -1) return -1;
    if (ib !== -1) return 1;
    return a.localeCompare(b);
  });

  return (
    <div className="ml-12 bg-white rounded-2xl shadow-sm border border-gray-100 transition-all hover:shadow-md hover:border-emerald-100 group overflow-hidden">

      {/* ── Pipeline header ── */}
      <div className="px-6 pt-5 pb-4">
        {/* Title row */}
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="flex items-center gap-1 px-2 py-0.5 bg-emerald-100 text-emerald-700 rounded-full text-[10px] font-bold uppercase tracking-widest">
              <Link2 className="w-3 h-3" /> Deployment Story
            </span>
            {event.link_type && (
              <span className="px-2 py-0.5 bg-violet-50 text-violet-600 rounded-full text-[10px] font-bold">
                {linkConfidenceLabel(event.link_type)}
              </span>
            )}
            {event.analysis_status === 'pending' && (
              <span className="flex items-center gap-1 px-2 py-0.5 bg-amber-50 text-amber-600 rounded-full border border-amber-100 text-[10px] font-bold">
                <Hourglass className="w-3 h-3" /> PENDING
              </span>
            )}
            {event.analysis_status === 'ready' && (
              <span className="flex items-center gap-1 px-2 py-0.5 bg-sky-50 text-sky-600 rounded-full border border-sky-100 text-[10px] font-bold">
                <Zap className="w-3 h-3" /> READY
              </span>
            )}
            {event.analysis_status === 'completed' && !analysis && (
              <span className="flex items-center gap-1 px-2 py-0.5 bg-emerald-50 text-emerald-600 rounded-full border border-emerald-100 text-[10px] font-bold">
                <ShieldCheck className="w-3 h-3" /> ANALYZED
              </span>
            )}
          </div>
          <button
            onClick={() => setStoryExpanded(v => !v)}
            className="p-1 rounded-full hover:bg-gray-100 text-gray-400 transition-colors"
            title={storyExpanded ? 'Collapse details' : 'Expand details'}
          >
            {storyExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </button>
        </div>

        {/* ── Stage pipeline ── */}
        <div className="flex items-stretch gap-0 overflow-x-auto pb-1">

          {/* Stage 1 – CI Build */}
          <div className="flex flex-col min-w-[120px] flex-1 bg-violet-50 border border-violet-100 rounded-xl p-3">
            <div className="flex items-center gap-1.5 text-[10px] font-bold text-violet-600 uppercase tracking-widest mb-2">
              <Bot className="w-3 h-3" /> CI Build
            </div>
            <p className="text-xs font-semibold text-gray-700 leading-snug line-clamp-2 mb-2">
              {intent.summary}
            </p>
            <div className="mt-auto flex flex-wrap gap-1">
              {intentSha && (
                <span className="flex items-center gap-0.5 px-1.5 py-0.5 bg-white text-violet-700 text-[10px] font-mono font-bold rounded border border-violet-100">
                  <GitBranch className="w-2.5 h-2.5" />
                  {shortSha(intentSha)}
                </span>
              )}
              <span className="flex items-center gap-0.5 px-1.5 py-0.5 bg-white text-gray-400 text-[10px] rounded border border-violet-100">
                <Clock className="w-2.5 h-2.5" />
                {timeAgo(new Date(intent.timestamp))}
              </span>
            </div>
          </div>

          {/* Connector */}
          <div className="flex items-center px-1 text-gray-300 self-center shrink-0">
            <ArrowRight className="w-4 h-4" />
          </div>

          {/* Stage 2 – Image Push (only if we have an image tag) */}
          {imageTag && (
            <>
              <div className="flex flex-col min-w-[120px] flex-1 bg-blue-50 border border-blue-100 rounded-xl p-3">
                <div className="flex items-center gap-1.5 text-[10px] font-bold text-blue-600 uppercase tracking-widest mb-2">
                  <PackageCheck className="w-3 h-3" /> Image Push
                </div>
                <p className="text-xs font-mono font-semibold text-blue-700 leading-snug break-all line-clamp-2 mb-2">
                  {shortImageTag(imageTag)}
                </p>
                {event.metadata?.previous_image_tag && (
                  <div className="mt-auto flex items-center gap-1 text-[10px] text-gray-400">
                    <span className="truncate max-w-[60px]" title={event.metadata.previous_image_tag}>
                      {shortImageTag(event.metadata.previous_image_tag)}
                    </span>
                    <ArrowRight className="w-2.5 h-2.5 shrink-0 text-blue-400" />
                    <span className="font-semibold text-blue-600 truncate max-w-[60px]" title={imageTag}>
                      {shortImageTag(imageTag)}
                    </span>
                  </div>
                )}
              </div>

              {/* Connector */}
              <div className="flex items-center px-1 text-gray-300 self-center shrink-0">
                <ArrowRight className="w-4 h-4" />
              </div>
            </>
          )}

          {/* Stage 3 – K8s Rollout */}
          <div className="flex flex-col min-w-[120px] flex-1 bg-emerald-50 border border-emerald-100 rounded-xl p-3">
            <div className="flex items-center gap-1.5 text-[10px] font-bold text-emerald-600 uppercase tracking-widest mb-2">
              <Box className="w-3 h-3" /> K8s Rollout
            </div>
            <p className="text-xs font-semibold text-gray-700 leading-snug line-clamp-2 mb-2">
              {event.summary}
            </p>
            <div className="mt-auto flex flex-wrap gap-1">
              {event.affected_services.slice(0, 2).map(s => (
                <Link key={s} href={`/services/${encodeURIComponent(s)}`} className="flex items-center gap-0.5 px-1.5 py-0.5 bg-white text-emerald-700 text-[10px] font-bold rounded border border-emerald-100 hover:bg-emerald-50 transition-colors">
                  <Tag className="w-2.5 h-2.5" />
                  {s}
                </Link>
              ))}
              <span className="flex items-center gap-0.5 px-1.5 py-0.5 bg-white text-gray-400 text-[10px] rounded border border-emerald-100">
                <Clock className="w-2.5 h-2.5" />
                {timeAgo(new Date(event.timestamp))}
              </span>
            </div>
          </div>

          {/* Connector */}
          <div className="flex items-center px-1 text-gray-300 self-center shrink-0">
            <ArrowRight className="w-4 h-4" />
          </div>

          {/* Stage 4 – Impact Score */}
          <div className={`flex flex-col min-w-[120px] flex-1 rounded-xl p-3 border ${
            analysis
              ? 'bg-gray-50 border-gray-200'
              : 'bg-gray-50 border-gray-200 border-dashed'
          }`}>
            <div className="flex items-center gap-1.5 text-[10px] font-bold text-gray-500 uppercase tracking-widest mb-2">
              <Zap className="w-3 h-3" /> Impact Score
            </div>

            {analysis ? (
              <div className="flex flex-col gap-1.5">
                <div className={`text-2xl font-black ${getImpactColor(analysis.impact_level)}`}>
                  {(analysis.impact_score * 100).toFixed(0)}
                  <span className="text-xs font-normal text-gray-400">%</span>
                </div>
                <ImpactBadge level={analysis.impact_level} score={analysis.impact_score} />
                <span className={`text-[10px] font-semibold ${analysis.confidence_score > 0.7 ? 'text-emerald-600' : 'text-orange-500'}`}>
                  <ShieldCheck className="w-2.5 h-2.5 inline mr-0.5" />
                  {(analysis.confidence_score * 100).toFixed(0)}% confidence
                </span>
              </div>
            ) : loading ? (
              <div className="flex items-center gap-1 text-blue-600 text-xs font-bold mt-1">
                <Loader2 className="w-3 h-3 animate-spin" /> Analyzing…
              </div>
            ) : isPending ? (
              <div className="flex items-center gap-1 text-amber-500 text-xs font-bold mt-1">
                <Hourglass className="w-3 h-3 animate-pulse" /> Pending
              </div>
            ) : (
              <button
                onClick={handleAnalyze}
                className={`mt-1 self-start flex items-center gap-1 px-2.5 py-1 text-xs font-bold rounded-lg transition-all shadow-sm ${
                  isCompleted
                    ? 'bg-emerald-600 text-white hover:bg-emerald-700 shadow-emerald-200'
                    : 'bg-blue-600 text-white hover:bg-blue-700 shadow-blue-200'
                }`}
              >
                {isCompleted ? <ShieldCheck className="w-3 h-3" /> : <Zap className="w-3 h-3 fill-current" />}
                {isCompleted ? 'Analyzed' : 'Analyze'}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* ── Expandable details ── */}
      {storyExpanded && (
        <div className="border-t border-gray-100 px-6 py-4 animate-in fade-in slide-in-from-top-1 duration-200">
          {/* Timestamps & metadata */}
          <div className="flex flex-wrap items-center gap-2 text-sm text-gray-400 mb-2">
            <Clock className="w-3 h-3" />
            {new Date(event.timestamp).toLocaleString()}
            {event.affected_services.map(s => (
              <Link key={s} href={`/services/${encodeURIComponent(s)}`} className="flex items-center gap-1 px-2 py-0.5 bg-emerald-50 text-emerald-600 text-[10px] font-bold rounded-full border border-emerald-100 hover:bg-emerald-100 transition-colors">
                <Tag className="w-2 h-2" /> {s}
              </Link>
            ))}
            {intentSha && (
              <span className="flex items-center gap-1 px-2 py-0.5 bg-gray-50 text-gray-600 text-[10px] font-bold rounded-full border border-gray-100"
                title={`Git SHA: ${intentSha}`}>
                <GitBranch className="w-2 h-2" />
                {shortSha(intentSha)}
              </span>
            )}
          </div>

          {event.blast_radius && <BlastRadiusDisplay blastRadius={event.blast_radius} />}

          {/* Merged metadata for both intent and execution, deduplicated */}
          {(() => {
            const merged: Record<string, string> = { ...intent.metadata, ...event.metadata };
            return Object.keys(merged).length > 0 ? <MetadataSection metadata={merged} /> : null;
          })()}

          {/* Intent contextual links */}
          {(intent.contextual_links?.length ?? 0) > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {intent.contextual_links!.map((link, i) => (
                <a key={i} href={link.url} target="_blank" rel="noopener noreferrer"
                  className="flex items-center gap-1.5 px-3 py-1 bg-violet-50 text-violet-700 text-xs font-semibold rounded-full hover:bg-violet-100 transition-colors border border-violet-100">
                  <ExternalLink className="w-3 h-3" /> {link.name}
                </a>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ── Analysis section ── */}
      {analysis && (
        <div className="border-t border-gray-100 px-6 pt-5 pb-0 animate-in fade-in slide-in-from-top-2 duration-300">
          {/* Score row — always visible, click to toggle details */}
          <button
            onClick={() => setDetailsExpanded(prev => !prev)}
            className="w-full flex items-center p-4 bg-gray-50 rounded-xl border border-gray-100 hover:bg-gray-100 transition-colors text-left mb-0"
          >
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
            <div className="flex-1 text-center border-r border-gray-200">
              <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Confidence</span>
              <div className={`text-xl font-black mt-1 flex items-center justify-center gap-1 ${analysis.confidence_score > 0.7 ? 'text-emerald-600' : 'text-orange-500'}`}>
                <ShieldCheck className="w-3 h-3" />
                {(analysis.confidence_score * 100).toFixed(0)}
                <span className="text-xs text-gray-400 font-normal">%</span>
              </div>
            </div>
            <div className="pl-4 text-gray-400 text-[10px] font-bold uppercase tracking-widest flex items-center gap-1">
              {detailsExpanded ? '▲ Hide' : '▼ Details'}
            </div>
          </button>

          {detailsExpanded && (
            <div className="mt-4 pb-5">
              {event.affected_services.length > 1 && (
                <div className="mb-4 flex items-center flex-wrap gap-2">
                  <span className="text-sm font-bold text-gray-500">Service:</span>
                  {event.affected_services.map(s => (
                    <button key={s} onClick={() => setSelectedService(s)}
                      className={`flex items-center gap-1 px-2 py-0.5 text-xs font-bold rounded-md ${selectedService === s ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-700'}`}>
                      <Tag className="w-2 h-2" /> {s}
                    </button>
                  ))}
                </div>
              )}
              <div className="flex justify-between items-center mb-3">
                <h4 className="text-xs font-bold text-gray-400 uppercase tracking-widest flex items-center gap-2">
                  <Activity className="w-3 h-3" /> Metric Shifts (vs Baseline)
                </h4>
                {!pageSelectedMetrics && (
                  <button onClick={() => setIsMetricModalOpen(true)}
                    className="flex items-center gap-2 px-3 py-1 bg-gray-100 text-gray-600 text-xs font-bold rounded-lg hover:bg-gray-200 transition-all border border-gray-200">
                    <Settings2 className="w-3 h-3" /> Customize
                  </button>
                )}
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3 mb-4">
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
                  const config = METRIC_CONFIG[metricName] || { label: metricName, description: `Custom metric: ${metricName}` };
                  return (
                    <MetricDelta
                      key={metricName}
                      label={config.label}
                      delta={allDeltas[metricName] || 0}
                      icon={getIcon(metricInfo?.icon)}
                      description={config.description}
                      reverse={config.reverse}
                    />
                  );
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* ── Contextual links (execution event) ── */}
      {((analysis?.change_event?.contextual_links?.length ?? 0) > 0 || (event.contextual_links?.length ?? 0) > 0) && (
        <div className="border-t border-gray-100 px-6 py-4 animate-in fade-in slide-in-from-top-2 duration-300">
          <h4 className="text-xs font-bold text-gray-400 uppercase tracking-widest mb-3">Contextual Links</h4>
          <div className="flex flex-wrap gap-2">
            {((analysis && analysis.change_event.contextual_links) || event.contextual_links)?.map((link, i) => (
              <a key={i} href={link.url} target="_blank" rel="noopener noreferrer"
                className="flex items-center gap-2 px-3 py-1 bg-blue-100 text-blue-800 text-xs font-semibold rounded-full hover:bg-blue-200 transition-colors shadow-sm">
                <ExternalLink className="w-3 h-3" /> {link.name}
              </a>
            ))}
          </div>
        </div>
      )}

      {/* ── Metric selection modal ── */}
      {isMetricModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex justify-center items-center z-50">
          <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md">
            <h3 className="text-lg font-bold mb-4">Customize Visible Metrics</h3>
            <div className="space-y-2">
              {availableMetrics.map(metric => (
                <label key={metric.name} className="flex items-center space-x-3">
                  <input type="checkbox" className="form-checkbox h-5 w-5 text-blue-600"
                    checked={selectedMetrics.has(metric.name)}
                    onChange={e => {
                      const next = new Set(selectedMetrics);
                      e.target.checked ? next.add(metric.name) : next.delete(metric.name);
                      setSelectedMetrics(next);
                    }}
                  />
                  <div className="flex items-center gap-2">
                    {getIcon(metric.icon)}
                    <span className="text-gray-700">{metric.name}</span>
                  </div>
                </label>
              ))}
            </div>
            <div className="mt-6 flex justify-end gap-2">
              <button onClick={() => setIsMetricModalOpen(false)}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200">
                Cancel
              </button>
              <button onClick={handleSavePreferences}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700">
                Save Preferences
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
