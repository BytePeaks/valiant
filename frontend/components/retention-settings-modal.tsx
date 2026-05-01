'use client';

import { useEffect, useState } from 'react';
import { X, Save, Clock, Check } from 'lucide-react';
import { fetchRetentionSettings, saveRetentionSettings } from '@/lib/api';

const PRESETS = [
  { label: '7 days', value: '7d' },
  { label: '30 days', value: '30d' },
  { label: '60 days', value: '60d' },
  { label: '90 days', value: '90d' },
  { label: '180 days', value: '180d' },
  { label: '1 year', value: '365d' },
];

interface RetentionSettingsModalProps {
  onClose: () => void;
}

export default function RetentionSettingsModal({ onClose }: RetentionSettingsModalProps) {
  const [eventTTL, setEventTTL] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchRetentionSettings()
      .then(s => setEventTTL(s.event_ttl))
      .catch(() => setEventTTL('90d'))
      .finally(() => setLoading(false));
  }, []);

  const handleSave = async () => {
    setError('');
    setSaving(true);
    try {
      await saveRetentionSettings(eventTTL.trim());
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/40 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-3xl shadow-2xl w-full max-w-md border border-gray-100">
        <div className="flex items-center justify-between p-6 border-b border-gray-100">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-blue-100 rounded-xl">
              <Clock className="w-4 h-4 text-blue-600" />
            </div>
            <div>
              <h2 className="text-lg font-black text-gray-900 uppercase italic tracking-tight">Retention Settings</h2>
              <p className="text-xs text-gray-400 font-medium">Manage how long events are kept</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-full hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-6 space-y-5">
          <div>
            <label className="block text-xs font-bold text-gray-500 uppercase tracking-widest mb-3">
              Event TTL
            </label>
            <div className="flex flex-wrap gap-2 mb-3">
              {PRESETS.map(p => (
                <button
                  key={p.value}
                  onClick={() => setEventTTL(p.value)}
                  className={`px-3 py-1 rounded-full text-xs font-bold border transition-all ${
                    eventTTL === p.value
                      ? 'bg-blue-600 border-blue-600 text-white'
                      : 'bg-white border-gray-200 text-gray-600 hover:border-blue-300'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
            <input
              type="text"
              value={loading ? '' : eventTTL}
              onChange={e => { setEventTTL(e.target.value); setSaved(false); }}
              placeholder="e.g. 90d, 168h"
              className="w-full px-4 py-2.5 rounded-xl border border-gray-200 text-sm text-gray-800 font-mono focus:outline-none focus:border-blue-300 focus:ring-1 focus:ring-blue-300 transition-all"
            />
            <p className="mt-1.5 text-xs text-gray-400">
              Use <code className="font-mono bg-gray-100 px-1 rounded">d</code> for days or <code className="font-mono bg-gray-100 px-1 rounded">h</code> for hours. Events older than this are deleted automatically.
            </p>
          </div>

          {error && (
            <p className="text-xs text-red-600 font-medium bg-red-50 px-3 py-2 rounded-lg">{error}</p>
          )}
        </div>

        <div className="flex justify-end gap-3 px-6 pb-6">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-bold text-gray-600 bg-gray-50 hover:bg-gray-100 rounded-xl border border-gray-200 transition-all"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving || loading}
            className={`flex items-center gap-2 px-4 py-2 text-sm font-bold rounded-xl border transition-all ${
              saved
                ? 'bg-emerald-50 border-emerald-200 text-emerald-600'
                : 'bg-blue-600 border-blue-600 text-white hover:bg-blue-700'
            }`}
          >
            {saved ? <Check className="w-4 h-4" /> : <Save className="w-4 h-4" />}
            {saved ? 'Saved!' : saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );
}
