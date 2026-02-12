import { MetricInfo } from '@/components/promql-modal';

export interface ContextualLink {
  name: string;
  url: string;
}

export interface ChangeEvent {
  id: string;
  source: string; // Deprecated in favor of trigger_type
  trigger_type?: string;
  is_intent?: boolean;
  is_execution?: boolean;
  execution_id?: string;
  change_type: string;
  timestamp: string;
  affected_services: string[];
  summary: string;
  metadata: Record<string, string>;
  analysis_status?: 'pending' | 'ready' | 'completed';
  contextual_links?: ContextualLink[];
  linked_intent?: ChangeEvent;       // The CI event that triggered this execution
  link_type?: string;                // "sha_match" or "image_tag_match"
  link_confidence?: number;          // 0.0 to 1.0
  link_reason?: string;              // Human-readable reason from link metadata
}

export interface MetricValues {
  error_rate: number;
  latency_p95_ms: number;
  rps: number;
  cpu: number;
  memory: number;
  additional_metrics?: Record<string, number>;
}

export interface ImpactAnalysis {
  change_event: ChangeEvent;
  baseline_metrics: MetricValues;
  impact_metrics: MetricValues;
  deltas: MetricValues;
  impact_score: number;
  impact_level: string;
  confidence_score: number;
}

export interface TimelineEventProps {
  event: ChangeEvent;
}

export interface EventFilters {
  limit?: number;
  offset?: number;
  service?: string;
  namespace?: string;
  from?: string;
  to?: string;
  search?: string;
  isExecutionOnly?: boolean;
  linked_only?: boolean;
}

export interface EventsResponse {
  events: ChangeEvent[];
  total: number;
  limit: number;
  offset: number;
}

export interface RankedChange {
  analysis: ImpactAnalysis;
  rank: number;
  likelihood_score: number;
  temporal_proximity: number;
  change_type_weight: number;
  service_scope: number;
}

export interface RankingsResponse {
  service: string;
  from: string;
  to: string;
  ranked: RankedChange[];
  total: number;
}

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

export async function fetchChangeEvents(
  filters?: EventFilters,
  signal?: AbortSignal
): Promise<EventsResponse> {
  // Map camelCase filter keys to snake_case backend query params
  const keyMap: Record<string, string> = {
    isExecutionOnly: 'is_execution_only',
  };

  const params = new URLSearchParams();
  if (filters) {
    for (const [key, value] of Object.entries(filters)) {
      if (value !== undefined && value !== null && value !== '') {
        const paramKey = keyMap[key] || key;
        if (typeof value === 'boolean') {
          params.set(paramKey, value.toString());
        } else {
          params.set(paramKey, String(value));
        }
      }
    }
  }
  const query = params.toString();
  const url = query ? `${API_BASE_URL}/events?${query}` : `${API_BASE_URL}/events`;
  const res = await fetch(url, { signal });
  if (!res.ok) {
    throw new Error('Failed to fetch events');
  }
  const data = await res.json();
  // Support both envelope and bare array responses
  if (Array.isArray(data)) {
    return { events: data, total: data.length, limit: data.length, offset: 0 };
  }
  return {
    events: data.events || [],
    total: data.total ?? 0,
    limit: data.limit ?? 0,
    offset: data.offset ?? 0,
  };
}

export async function fetchServices(namespace?: string, linkedOnly?: boolean): Promise<string[]> {
  const params = new URLSearchParams();
  if (namespace) params.set('namespace', namespace);
  if (linkedOnly) params.set('linked_only', 'true');
  const query = params.toString();
  const url = query ? `${API_BASE_URL}/services?${query}` : `${API_BASE_URL}/services`;
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error('Failed to fetch services');
  }
  const data = await res.json();
  return data || [];
}

export async function fetchNamespaces(): Promise<string[]> {
  const res = await fetch(`${API_BASE_URL}/namespaces`);
  if (!res.ok) {
    throw new Error('Failed to fetch namespaces');
  }
  const data = await res.json();
  return data || [];
}

export async function fetchAvailableMetrics(): Promise<MetricInfo[]> {
  const res = await fetch(`${API_BASE_URL}/metrics`);
  if (!res.ok) {
    throw new Error('Failed to fetch available metrics');
  }
  const data = await res.json();
  return data || [];
}

export async function analyzeImpact(eventId: string): Promise<ImpactAnalysis> {
  const res = await fetch(`${API_BASE_URL}/analyze`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ event_id: eventId }),
  });
  if (!res.ok) {
    throw new Error('Failed to analyze impact');
  }
  return res.json();
}

export async function fetchServicePreferences(serviceName: string): Promise<string[]> {
  const res = await fetch(`${API_BASE_URL}/services/${serviceName}/preferences`);
  if (!res.ok) {
    if (res.status === 404) {
      return []; // No preferences saved yet
    }
    throw new Error('Failed to fetch service preferences');
  }
  return res.json();
}

export async function saveServicePreferences(serviceName: string, visibleMetrics: string[]): Promise<void> {
  const res = await fetch(`${API_BASE_URL}/services/${serviceName}/preferences`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(visibleMetrics),
  });
  if (!res.ok) {
    throw new Error('Failed to save service preferences');
  }
}

export async function fetchRankings(
  service: string,
  from: string,
  to: string
): Promise<RankingsResponse> {
  const params = new URLSearchParams({ service, from, to });
  const res = await fetch(`${API_BASE_URL}/rankings?${params}`);
  if (!res.ok) {
    throw new Error('Failed to fetch rankings');
  }
  return res.json();
}
