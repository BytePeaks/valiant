import React from 'react';
import { ExternalLink } from 'lucide-react';

const URL_RE = /^https?:\/\/[^\s]+$/;
const INLINE_RE = /(\[([^\]]+)\]\((https?:\/\/[^)]+)\)|\*\*([^*]+)\*\*|\*([^*]+)\*|`([^`]+)`)/g;

function isUrl(value: string): boolean {
  return URL_RE.test(value.trim());
}

function parseInline(text: string): React.ReactNode[] {
  const nodes: React.ReactNode[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  INLINE_RE.lastIndex = 0;

  while ((match = INLINE_RE.exec(text)) !== null) {
    if (match.index > last) {
      nodes.push(text.slice(last, match.index));
    }
    if (match[2] && match[3]) {
      // [text](url)
      nodes.push(
        <a key={match.index} href={match[3]} target="_blank" rel="noopener noreferrer"
          className="inline-flex items-center gap-0.5 text-blue-600 underline hover:text-blue-800">
          {match[2]}<ExternalLink className="w-2.5 h-2.5 inline" />
        </a>
      );
    } else if (match[4]) {
      nodes.push(<strong key={match.index} className="font-semibold text-gray-800">{match[4]}</strong>);
    } else if (match[5]) {
      nodes.push(<em key={match.index} className="italic">{match[5]}</em>);
    } else if (match[6]) {
      nodes.push(
        <code key={match.index} className="px-1 py-0.5 bg-gray-100 rounded text-[10px] font-mono text-gray-700">
          {match[6]}
        </code>
      );
    }
    last = match.index + match[0].length;
  }
  if (last < text.length) nodes.push(text.slice(last));
  return nodes;
}

interface MetadataValueProps {
  value: string;
}

export function MetadataValue({ value }: MetadataValueProps) {
  if (isUrl(value)) {
    return (
      <a href={value} target="_blank" rel="noopener noreferrer"
        className="inline-flex items-center gap-1 text-blue-600 underline hover:text-blue-800 break-all">
        {value}
        <ExternalLink className="w-3 h-3 shrink-0" />
      </a>
    );
  }
  const nodes = parseInline(value);
  return <span>{nodes}</span>;
}

const HIDDEN_KEYS = new Set([
  'git_commit_sha', 'git_sha', 'image_tag', 'previous_image_tag',
]);

interface MetadataSectionProps {
  metadata: Record<string, string>;
}

export function MetadataSection({ metadata }: MetadataSectionProps) {
  const entries = Object.entries(metadata).filter(([k]) => !HIDDEN_KEYS.has(k));
  if (entries.length === 0) return null;

  return (
    <div className="mt-3 pt-3 border-t border-gray-100">
      <h4 className="text-[10px] font-bold text-gray-400 uppercase tracking-widest mb-2">Metadata</h4>
      <dl className="grid grid-cols-1 gap-1">
        {entries.map(([key, value]) => (
          <div key={key} className="flex items-start gap-2 text-xs">
            <dt className="shrink-0 font-mono text-gray-400 min-w-[100px] pt-px">{key}</dt>
            <dd className="text-gray-700 break-words min-w-0">
              <MetadataValue value={value} />
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
