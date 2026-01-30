'use client';

import { useState } from 'react';
import { ChangeEvent, ImpactAnalysis, analyzeImpact } from '@/lib/api';

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

  return (
    <div className="ml-12 p-4 bg-white rounded-lg shadow-sm border border-gray-100 transition-all hover:border-blue-200">
      <div className="flex justify-between items-start">
        <div>
          <span className="text-xs font-semibold text-blue-600 uppercase tracking-wider">{event.source}</span>
          <h3 className="text-lg font-bold text-gray-900">{event.summary}</h3>
          <p className="text-sm text-gray-500">{new Date(event.timestamp).toLocaleString()}</p>
        </div>
        {!analysis && !loading && (
          <button 
            onClick={handleAnalyze}
            className="px-3 py-1 bg-blue-600 text-white text-xs rounded hover:bg-blue-700 transition-colors"
          >
            Analyze Impact
          </button>
        )}
        {loading && <div className="text-xs text-blue-600 animate-pulse">Analyzing...</div>}
      </div>

      {analysis && (
        <div className="mt-4 pt-4 border-t border-gray-100 grid grid-cols-2 gap-4">
          <div>
            <span className="text-xs text-gray-400 uppercase">Impact Score</span>
            <div className={`text-2xl font-black ${getImpactColor(analysis.impact_level)}`}>
              {(analysis.impact_score * 100).toFixed(0)}%
            </div>
          </div>
          <div>
            <span className="text-xs text-gray-400 uppercase">Level</span>
            <div className={`text-lg font-bold ${getImpactColor(analysis.impact_level)}`}>
              {analysis.impact_level}
            </div>
          </div>
        </div>
      )}
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
