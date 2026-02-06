'use client';

import React, { useState } from 'react';
import { X, Copy, Check, Info } from 'lucide-react';

export interface MetricInfo {
  name: string;
  description: string;
  promql?: string;
  query?: string;
  icon?: string;
}

interface PromQLModalProps {
  isOpen: boolean;
  onClose: () => void;
  metrics: MetricInfo[];
  customMetrics: MetricInfo[];
}

type Tab = 'builtin' | 'custom';

export default function PromQLModal({ isOpen, onClose, metrics, customMetrics }: PromQLModalProps) {
  const [copiedPromQL, setCopiedPromQL] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('builtin');

  if (!isOpen) return null;

  const handleCopy = (query: string) => {
    navigator.clipboard.writeText(query);
    setCopiedPromQL(query);
    setTimeout(() => setCopiedPromQL(null), 2000); // Reset copied state after 2 seconds
  };

  const renderMetricsList = (list: MetricInfo[]) => {
    if (list.length === 0) {
      return (
        <div className="text-center text-gray-500 py-8">
          No custom metrics defined for this service.
        </div>
      );
    }

    return list.map((metric, index) => {
      const query = metric.query || metric.promql || '';
      return (
        <div key={index} className="mb-6 last:mb-0 p-4 border border-gray-200 rounded-lg bg-gray-50">
          <h4 className="font-semibold text-gray-700 text-base mb-1">{metric.name}</h4>
          <p className="text-sm text-gray-500 mb-3">{metric.description}</p>
          <div className="relative bg-gray-800 rounded-md p-3 font-mono text-white text-xs flex items-center justify-between">
            <code className="flex-1 overflow-x-auto whitespace-pre-wrap pr-8">
              {query}
            </code>
            <button
              onClick={() => handleCopy(query)}
              className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-md text-gray-400 hover:bg-gray-700 hover:text-white transition-colors"
              title="Copy PromQL"
            >
              {copiedPromQL === query ? (
                <Check className="w-4 h-4 text-green-400" />
              ) : (
                <Copy className="w-4 h-4" />
              )}
            </button>
          </div>
        </div>
      );
    });
  };

  return (
    <div className="fixed inset-0 bg-gray-600 bg-opacity-50 flex justify-center items-center p-4 z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] flex flex-col">
        <div className="flex justify-between items-center p-4 border-b border-gray-200">
          <h3 className="text-xl font-bold text-gray-800 flex items-center gap-2">
            <Info className="w-5 h-5 text-blue-600" />
            Prometheus Metrics
          </h3>
          <button onClick={onClose} className="p-2 rounded-full hover:bg-gray-100 text-gray-400 hover:text-gray-600">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="px-4 pt-4 border-b border-gray-200">
          <div className="flex gap-4">
            <button
              onClick={() => setActiveTab('builtin')}
              className={`pb-2 text-sm font-semibold ${activeTab === 'builtin' ? 'text-blue-600 border-b-2 border-blue-600' : 'text-gray-500 hover:text-gray-700'}`}
            >
              Built-in
            </button>
            {customMetrics.length > 0 && (
              <button
                onClick={() => setActiveTab('custom')}
                className={`pb-2 text-sm font-semibold ${activeTab === 'custom' ? 'text-blue-600 border-b-2 border-blue-600' : 'text-gray-500 hover:text-gray-700'}`}
              >
                Custom
              </button>
            )}
          </div>
        </div>

        <div className="p-4 overflow-y-auto flex-1">
          {activeTab === 'builtin' ? renderMetricsList(metrics) : renderMetricsList(customMetrics)}
        </div>

        <div className="p-4 border-t border-gray-200 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors font-semibold"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
