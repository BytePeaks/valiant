'use client';

import React from 'react';
import { AlertCircle, Clock, Cpu, Database, Zap } from 'lucide-react';

export function MetricDelta({ label, delta, reverse = false, icon, description }: { label: string, delta: number, reverse?: boolean, icon: React.ReactNode, description: string }) {
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