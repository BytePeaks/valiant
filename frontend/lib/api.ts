export interface ChangeEvent {
  id: string;
  source: string;
  summary: string;
  timestamp: string;
}

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

export async function fetchChangeEvents(): Promise<ChangeEvent[]> {
  const res = await fetch(`${API_BASE_URL}/events`);
  if (!res.ok) {
    throw new Error('Failed to fetch events');
  }
  return res.json();
}
