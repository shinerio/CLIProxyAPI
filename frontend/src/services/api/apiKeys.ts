/**
 * API 密钥管理
 */

import type { ClientApiKeyConfig } from '@/types';
import { apiClient } from './client';

const normalizeApiKeys = (input: unknown): ClientApiKeyConfig[] => {
  if (!Array.isArray(input)) return [];
  const seen = new Set<string>();
  const result: ClientApiKeyConfig[] = [];

  input.forEach((item) => {
    const record =
      item !== null && typeof item === 'object' && !Array.isArray(item)
        ? (item as Record<string, unknown>)
        : null;
    const keyRaw =
      typeof item === 'string' ? item : (record?.key ?? record?.['api-key'] ?? record?.apiKey ?? '');
    const key = String(keyRaw ?? '').trim();
    if (!key || seen.has(key)) return;
    seen.add(key);

    const nameRaw = record?.name;
    const name = typeof nameRaw === 'string' ? nameRaw.trim() : '';
    const allowedRaw =
      record?.['allowed-auth-indices'] ??
      record?.allowedAuthIndices ??
      record?.['allowed_auth_indices'] ??
      [];
    const allowedAuthIndices = Array.isArray(allowedRaw)
      ? Array.from(
          new Set(
            allowedRaw.map((value) => String(value ?? '').trim()).filter(Boolean)
          )
        )
      : [];

    result.push({
      ...(name ? { name } : {}),
      key,
      ...(allowedAuthIndices.length ? { allowedAuthIndices } : {}),
    });
  });

  return result;
};

export const apiKeysApi = {
  async list(): Promise<ClientApiKeyConfig[]> {
    const data = await apiClient.get<Record<string, unknown>>('/api-keys');
    const keys = data['api-keys'] ?? data.apiKeys;
    return normalizeApiKeys(keys);
  },

  replace: (keys: Array<string | ClientApiKeyConfig>) => apiClient.put('/api-keys', keys),

  update: (index: number, value: string | ClientApiKeyConfig) =>
    apiClient.patch('/api-keys', { index, value }),

  delete: (index: number) => apiClient.delete(`/api-keys?index=${index}`)
};
