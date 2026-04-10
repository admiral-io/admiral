export function parseLabelsFromString(input: string): Record<string, string> | undefined {
  const trimmed = input.trim();
  if (!trimmed) return undefined;
  const map: Record<string, string> = {};
  for (const part of trimmed.split(',')) {
    const [k, ...rest] = part.split('=').map((s) => s.trim());
    if (k) map[k] = rest.join('=') ?? '';
  }
  return Object.keys(map).length ? map : undefined;
}

export function formatLabelsToString(labels?: Record<string, string>): string {
  if (!labels || !Object.keys(labels).length) return '';
  return Object.entries(labels)
    .map(([k, v]) => `${k}=${v}`)
    .join(', ');
}

export function formatShortDate(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}
