'use client';

import { useState } from 'react';
import { ChangeEvent, ImpactAnalysis, analyzeImpact } from '@/lib/api';
import { 
  Zap, 
  Clock, 
  Activity, 
  Cpu, 
  Database, 
  AlertCircle, 
  CheckCircle2, 
  Info,
  GitBranch,
  Box,
  Terminal,
  ChevronRight,
  Loader2
} from 'lucide-react';

interface TimelineEventProps {
  event: ChangeEvent;
}

export default function TimelineEvent({ event }: TimelineEventProps) {
  const [analysis, setAnalysis] = useState<ImpactAnalysis | null>(null);
  const [loading, setLoading] = useState(false);

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

  const getSourceIcon = () => {
    switch (event.source) {
      case 'kubernetes': return <Box className="w-4 h-4" />;
      case 'git': return <GitBranch className="w-4 h-4" />;
      case 'ci-cd': return <Terminal className="w-4 h-4" />;
      default: return <Info className="w-4 h-4" />;
    }
  };

  return (
    <div className="ml-12 p-6 bg-white rounded-2xl shadow-sm border border-gray-100 transition-all hover:shadow-md hover:border-blue-100 group">
      <div className="flex justify-between items-start">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-[10px] font-bold text-blue-600 uppercase tracking-widest">
            {getSourceIcon()}
            {event.source}
          </div>
          <h3 className="text-xl font-bold text-gray-900 group-hover:text-blue-700 transition-colors">{event.summary}</h3>
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Clock className="w-3 h-3" />
            {new Date(event.timestamp).toLocaleString()}
            <span className="flex items-center gap-1 px-2 py-0.5 bg-blue-600 text-white text-[10px] font-bold rounded-full shadow-sm">
              <Clock className="w-2 h-2" />
              {timeAgo(new Date(event.timestamp))}
            </span>
          </div>
        </div>
        {!analysis && !loading && (
          <button 
            onClick={handleAnalyze}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white text-sm font-bold rounded-xl hover:bg-blue-700 transition-all shadow-lg shadow-blue-200"
          >
            <Zap className="w-4 h-4 fill-current" />
            Analyze
          </button>
        )}
        {loading && (
          <div className="flex items-center gap-2 px-4 py-2 text-blue-600 font-bold text-sm">
            <Loader2 className="w-4 h-4 animate-spin" />
            Analyzing...
          </div>
        )}
      </div>

      {analysis && (
        <div className="mt-6 pt-6 border-t border-gray-100 animate-in fade-in slide-in-from-top-2 duration-300">
          
          <div className="flex items-center p-4 bg-gray-50 rounded-xl mb-6 border border-gray-100">
             <div className="flex-1 text-center border-r border-gray-200 pr-4">
                <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Impact Score</span>
                <div className={`text-3xl font-black mt-1 ${getImpactColor(analysis.impact_level)}`}>
                  {(analysis.impact_score * 100).toFixed(0)}<span className="text-sm text-gray-400 font-normal">%</span>
                </div>
             </div>
             <div className="flex-1 text-center pl-4">
                <span className="text-[10px] font-bold text-gray-400 uppercase tracking-widest">Severity Level</span>
                <div className={`text-xl font-bold mt-1 px-3 py-1 inline-block rounded-full bg-white border border-gray-200 shadow-sm ${getImpactColor(analysis.impact_level)}`}>
                  {analysis.impact_level}
                </div>
             </div>
          </div>

          <div>
            <h4 className="text-xs font-bold text-gray-400 uppercase tracking-widest mb-3 flex items-center gap-2">
              <Activity className="w-3 h-3" />
              Metric Shifts (vs Baseline)
            </h4>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
               <MetricDelta 
                 label="Errors" 
                 delta={analysis.deltas.error_rate} 
                 icon={<AlertCircle className="w-3 h-3" />} 
                 description="Percentage of failed requests (5xx). A positive value means more errors."
               />
               <MetricDelta 
                 label="Latency" 
                 delta={analysis.deltas.latency_p95_ms} 
                 icon={<Clock className="w-3 h-3" />} 
                 description="95th percentile response time. A positive value means the service is slower."
               />
               <MetricDelta 
                 label="RPS" 
                 delta={analysis.deltas.rps} 
                 reverse 
                 icon={<Zap className="w-3 h-3" />} 
                 description="Requests Per Second. A negative value means traffic has dropped."
               />
               <MetricDelta 
                 label="CPU" 
                 delta={analysis.deltas.cpu_saturation_percent} 
                 icon={<Cpu className="w-3 h-3" />} 
                 description="CPU usage relative to limit. High increase indicates inefficiency."
               />
               <MetricDelta 
                 label="Memory" 
                 delta={analysis.deltas.memory_saturation_percent} 
                 icon={<Database className="w-3 h-3" />} 
                 description="Memory usage relative to limit. High increase risks OOM kills."
               />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function MetricDelta({ label, delta, reverse = false, icon, description }: { label: string, delta: number, reverse?: boolean, icon: React.ReactNode, description: string }) {
  const isPositive = delta > 0;
  const isNegative = delta < 0;
  const isZero = delta === 0;
  
  const percentage = (delta * 100).toFixed(1);
  const displayValue = isPositive ? `+${percentage}%` : `${percentage}%`;

  let valueColor = 'text-gray-400';
  let bgColor = 'bg-gray-50';
  let borderColor = 'border-gray-100';

  if (!isZero) {
    const isGood = reverse ? isPositive : isNegative;
    if (isGood) {
      valueColor = 'text-emerald-600';
      bgColor = 'bg-emerald-50/50';
      borderColor = 'border-emerald-100';
    } else {
      valueColor = 'text-rose-600';
      bgColor = 'bg-rose-50/50';
      borderColor = 'border-rose-100';
    }
  }

  return (
    <div className={`group/card relative flex flex-col items-center justify-center p-3 rounded-xl border ${borderColor} ${bgColor} transition-all hover:scale-105 cursor-help`}>
      <div className="text-gray-400 mb-1">{icon}</div>
      <span className="text-[9px] font-bold text-gray-500 uppercase mb-1">{label}</span>
      <span className={`text-xs font-mono font-black ${valueColor}`}>
        {displayValue}
      </span>
      
      {/* Tooltip */}
      <div className="absolute bottom-full mb-2 hidden group-hover/card:block w-32 p-2 bg-gray-900 text-white text-[10px] rounded shadow-lg z-10 text-center pointer-events-none">
        {description}
        <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
      </div>
    </div>
  );
}

function getImpactColor(level: string) {
  switch (level) {
    case 'HIGH': return 'text-red-600';
    case 'MEDIUM': return 'text-orange-500';
    case 'LOW': return 'text-yellow-500';
    default: return 'text-green-500';
  }
}

function timeAgo(date: Date) {
  const seconds = Math.floor((new Date().getTime() - date.getTime()) / 1000);
  
  let interval = seconds / 31536000;
  if (interval > 1) return Math.floor(interval) + "y ago";
  
  interval = seconds / 2592000;
  if (interval > 1) return Math.floor(interval) + "mo ago";
  
  interval = seconds / 86400;
  if (interval > 1) return Math.floor(interval) + "d ago";
  
  interval = seconds / 3600;
  if (interval > 1) return Math.floor(interval) + "h ago";
  
  interval = seconds / 60;
  if (interval > 1) return Math.floor(interval) + "m ago";
  
  return Math.floor(seconds) + "s ago";
}
