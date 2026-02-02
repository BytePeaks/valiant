export interface ChangeEvent {
  id: string;
  source: string; // Deprecated in favor of trigger_type
  trigger_type?: string;
  execution_id?: string;
  change_type: string;
  timestamp: string;
  affected_services: string[];
  summary: string;
  metadata: Record<string, string>;
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

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

export async function fetchChangeEvents(): Promise<ChangeEvent[]> {
  const res = await fetch(`${API_BASE_URL}/events`);
  if (!res.ok) {
    throw new Error('Failed to fetch events');
  }
  const data = await res.json();
  return data || [];
}

export async function fetchServices(): Promise<string[]> {
  const res = await fetch(`${API_BASE_URL}/services`);
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


export interface MetricInfo {
  name: string;
  icon?: string;
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
