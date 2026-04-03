import { isMap, parseDocument } from 'yaml';
import { makeClientId } from '@/types/visualConfig';
import type { VisualApiKeyEntry } from '@/types/visualConfig';

type YamlDocument = ReturnType<typeof parseDocument>;
type YamlPath = string[];

function asRecord(value: unknown): Record<string, unknown> | null {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function extractApiKeyValue(raw: unknown): string | null {
  if (typeof raw === 'string') {
    const trimmed = raw.trim();
    return trimmed ? trimmed : null;
  }

  const record = asRecord(raw);
  if (!record) return null;

  const candidates = [record['api-key'], record.apiKey, record.key, record.Key];
  for (const candidate of candidates) {
    if (typeof candidate === 'string') {
      const trimmed = candidate.trim();
      if (trimmed) return trimmed;
    }
  }

  return null;
}

function parseAllowedAuthIndices(raw: unknown): string[] {
  const source = Array.isArray(raw) ? raw : typeof raw === 'string' ? raw.split(/[\n,]+/) : [];
  const seen = new Set<string>();
  const values: string[] = [];
  source.forEach((item) => {
    const trimmed = String(item ?? '').trim();
    if (!trimmed || seen.has(trimmed)) return;
    seen.add(trimmed);
    values.push(trimmed);
  });
  return values;
}

function parseApiKeys(raw: unknown): VisualApiKeyEntry[] {
  if (!Array.isArray(raw)) return [];

  const entries: VisualApiKeyEntry[] = [];
  for (const item of raw) {
    const key = extractApiKeyValue(item);
    if (!key) continue;
    const record = asRecord(item);
    const name = typeof record?.name === 'string' ? record.name.trim() : '';
    const allowedAuthIndices = parseAllowedAuthIndices(
      record?.['allowed-auth-indices'] ??
        record?.allowedAuthIndices ??
        record?.['allowed_auth_indices'] ??
        record?.['allowed-auths'] ??
        record?.allowedAuths
    );
    entries.push({
      id: makeClientId(),
      name,
      key,
      allowedAuthIndices,
    });
  }
  return entries;
}

function docHas(doc: YamlDocument, path: YamlPath): boolean {
  return doc.hasIn(path);
}

function deleteIfMapEmpty(doc: YamlDocument, path: YamlPath): void {
  const value = doc.getIn(path, true);
  if (!isMap(value)) return;
  if (value.items.length === 0) doc.deleteIn(path);
}

function setStringInDoc(doc: YamlDocument, path: YamlPath, value: string): void {
  if (value.trim()) {
    doc.setIn(path, value);
    return;
  }
  if (docHas(doc, path)) {
    doc.deleteIn(path);
  }
}

function deleteLegacyApiKeysProvider(doc: YamlDocument): void {
  if (docHas(doc, ['auth', 'providers', 'config-api-key', 'api-key-entries'])) {
    doc.deleteIn(['auth', 'providers', 'config-api-key', 'api-key-entries']);
  }
  if (docHas(doc, ['auth', 'providers', 'config-api-key', 'api-keys'])) {
    doc.deleteIn(['auth', 'providers', 'config-api-key', 'api-keys']);
  }
  deleteIfMapEmpty(doc, ['auth', 'providers', 'config-api-key']);
  deleteIfMapEmpty(doc, ['auth', 'providers']);
  deleteIfMapEmpty(doc, ['auth']);
}

export function resolveClientAuthValues(parsed: Record<string, unknown>): {
  authDir: string;
  apiKeys: VisualApiKeyEntry[];
} {
  if (Object.prototype.hasOwnProperty.call(parsed, 'api-keys')) {
    return {
      authDir: typeof parsed['auth-dir'] === 'string' ? parsed['auth-dir'] : '',
      apiKeys: parseApiKeys(parsed['api-keys']),
    };
  }

  const auth = asRecord(parsed.auth);
  const providers = asRecord(auth?.providers);
  const configApiKeyProvider = asRecord(providers?.['config-api-key']);

  return {
    authDir: typeof parsed['auth-dir'] === 'string' ? parsed['auth-dir'] : '',
    apiKeys: configApiKeyProvider
      ? parseApiKeys(
          Object.prototype.hasOwnProperty.call(configApiKeyProvider, 'api-key-entries')
            ? configApiKeyProvider['api-key-entries']
            : configApiKeyProvider['api-keys']
        )
      : [],
  };
}

export function applyClientAuthChangesToYaml(
  currentYaml: string,
  authDir: string,
  apiKeys: VisualApiKeyEntry[]
): string {
  try {
    const doc = parseDocument(currentYaml);
    if (doc.errors.length > 0) return currentYaml;
    if (!isMap(doc.contents)) {
      doc.contents = doc.createNode({}) as unknown as typeof doc.contents;
    }

    setStringInDoc(doc, ['auth-dir'], authDir);

    const normalizedApiKeys = apiKeys
      .map((entry) => ({
        name: entry.name.trim(),
        key: entry.key.trim(),
        allowedAuthIndices: entry.allowedAuthIndices.map((item) => item.trim()).filter(Boolean),
      }))
      .filter((entry) => entry.key);

    if (normalizedApiKeys.length > 0) {
      doc.setIn(
        ['api-keys'],
        normalizedApiKeys.map((entry) =>
          entry.name || entry.allowedAuthIndices.length > 0
            ? {
                ...(entry.name ? { name: entry.name } : {}),
                key: entry.key,
                ...(entry.allowedAuthIndices.length > 0
                  ? { 'allowed-auth-indices': entry.allowedAuthIndices }
                  : {}),
              }
            : entry.key
        )
      );
    } else if (docHas(doc, ['api-keys'])) {
      doc.deleteIn(['api-keys']);
    }

    deleteLegacyApiKeysProvider(doc);

    return doc.toString({ indent: 2, lineWidth: 120, minContentWidth: 0 });
  } catch {
    return currentYaml;
  }
}
