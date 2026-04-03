import type { VisualApiKeyEntry } from '@/types/visualConfig';

export type ClientAuthScopeFilter = 'all' | 'restricted' | 'unrestricted';
export type ClientAuthSort = 'default' | 'name';

export function isRestrictedClientKey(entry: VisualApiKeyEntry): boolean {
  return Array.isArray(entry.allowedAuthIndices) && entry.allowedAuthIndices.length > 0;
}

export function matchesClientAuthSearch(entry: VisualApiKeyEntry, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;

  return (
    entry.name.toLowerCase().includes(needle) ||
    entry.key.toLowerCase().includes(needle) ||
    entry.allowedAuthIndices.some((item) => item.toLowerCase().includes(needle))
  );
}
