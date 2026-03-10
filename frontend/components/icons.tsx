
import { icons, Activity } from 'lucide-react';
import React from 'react';

export const getIcon = (name?: string) => {
  if (name && icons[name as keyof typeof icons]) {
    const LucideIcon = icons[name as keyof typeof icons];
    return <LucideIcon className="w-3 h-3" />;
  }
  return <Activity className="w-3 h-3" />;
};
