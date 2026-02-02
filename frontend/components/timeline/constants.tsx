// frontend/components/timeline/constants.ts
export const METRIC_CONFIG: { [key: string]: { label: string; description: string; reverse?: boolean } } = {
  error_rate: { label: 'Errors', description: 'Percentage of failed requests (5xx).' },
  latency_p95_ms: { label: 'Latency', description: '95th percentile response time.' },
  rps: { label: 'RPS', description: 'Requests Per Second.', reverse: true },
  cpu: { label: 'CPU', description: 'CPU usage relative to limit.' },
  memory: { label: 'Memory', description: 'Memory usage relative to limit.' },
};

export const CORE_METRICS = ["error_rate", "latency_p95_ms", "rps", "cpu", "memory"];
