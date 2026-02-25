
'use client';

import { useState } from 'react';
import { BlastRadius } from '@/lib/api';
import { ChevronDown, ChevronRight, Layers, Database } from 'lucide-react';
import Link from 'next/link';

interface BlastRadiusProps {
  blastRadius: BlastRadius;
}

export default function BlastRadiusDisplay({ blastRadius }: BlastRadiusProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  if (!blastRadius || blastRadius.total_workloads === 0) {
    return null;
  }

  return (
    <div className="mt-2 text-sm">
      <div className="flex items-center gap-2">
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="flex items-center gap-1 text-gray-500 hover:text-gray-700"
        >
          {isExpanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
          <span className="font-bold">Blast Radius:</span>
        </button>
        <span className="px-2 py-1 bg-red-100 text-red-800 text-xs font-bold rounded-full">
          {blastRadius.total_workloads} affected workload(s)
        </span>
      </div>

      {isExpanded && (
        <div className="mt-2 pl-6 border-l-2 border-gray-200 ml-1">
          {blastRadius.affected_deployments.length > 0 && (
            <div>
              <h4 className="font-semibold text-gray-700 flex items-center gap-1"><Layers className="w-3 h-3" />Deployments:</h4>
              <ul className="list-disc pl-5 mt-1 space-y-1">
                {blastRadius.affected_deployments.map(dep => (
                  <li key={dep}>
                    <Link href={`/services/${encodeURIComponent(dep)}`} className="text-blue-600 hover:underline">
                      {dep}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {blastRadius.affected_statefulsets.length > 0 && (
            <div className="mt-2">
              <h4 className="font-semibold text-gray-700 flex items-center gap-1"><Database className="w-3 h-3" />StatefulSets:</h4>
              <ul className="list-disc pl-5 mt-1 space-y-1">
                {blastRadius.affected_statefulsets.map(ss => (
                  <li key={ss}>
                    <Link href={`/services/${encodeURIComponent(ss)}`} className="text-blue-600 hover:underline">
                      {ss}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
