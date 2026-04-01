import { memo, useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { Select } from '@/components/ui/Select';
import { authFilesApi } from '@/services/api';
import { useConfigStore, useNotificationStore } from '@/stores';
import styles from './VisualConfigEditor.module.scss';
import { copyToClipboard } from '@/utils/clipboard';
import type {
  PayloadFilterRule,
  PayloadModelEntry,
  PayloadParamEntry,
  PayloadParamValidationErrorCode,
  PayloadParamValueType,
  PayloadRule,
  VisualApiKeyEntry,
} from '@/types/visualConfig';
import { makeClientId } from '@/types/visualConfig';
import type { AuthFileItem, Config } from '@/types';
import {
  getPayloadParamValidationError,
  VISUAL_CONFIG_PAYLOAD_VALUE_TYPE_OPTIONS,
  VISUAL_CONFIG_PROTOCOL_OPTIONS,
} from '@/hooks/useVisualConfig';
import { maskApiKey } from '@/utils/format';
import { isValidApiKeyCharset } from '@/utils/validation';

/** Minimum character count before the expand/collapse toggle appears. */
const EXPAND_THRESHOLD = 30;

/** Auto-expanding textarea that collapses back to a single-line input on demand. */
function ExpandableInput({
  value,
  placeholder,
  ariaLabel,
  disabled,
  className,
  onChange,
}: {
  value: string;
  placeholder?: string;
  ariaLabel?: string;
  disabled?: boolean;
  className?: string;
  onChange: (nextValue: string) => void;
}) {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(true);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const autoResize = useCallback((el: HTMLTextAreaElement) => {
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, []);

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    // Strip newlines — these fields are single-line identifiers/paths that
    // would break YAML serialization if they contained line breaks.
    const sanitized = e.target.value.replace(/[\r\n]/g, '');
    onChange(sanitized);
    // autoResize is handled by useLayoutEffect after React syncs the
    // sanitized value back to the DOM — calling it here would measure
    // stale content.
  };

  // Resize synchronously before paint to avoid visual flicker.
  useLayoutEffect(() => {
    if (!collapsed && textareaRef.current) {
      autoResize(textareaRef.current);
    }
  }, [collapsed, value, autoResize]);

  if (collapsed) {
    return (
      <div className={styles.expandableInputWrapper}>
        <input
          className={`input ${className ?? ''}`}
          placeholder={placeholder}
          aria-label={ariaLabel}
          value={value}
          onChange={(e) => onChange(e.target.value.replace(/[\r\n]/g, ''))}
          disabled={disabled}
        />
        {value.length > EXPAND_THRESHOLD && (
          <button
            type="button"
            className={styles.expandableToggle}
            disabled={disabled}
            onClick={() => {
              setCollapsed(false);
              requestAnimationFrame(() => {
                textareaRef.current?.focus();
              });
            }}
            title={t('common.expand')}
            aria-label={t('common.expand')}
          >
            ▼
          </button>
        )}
      </div>
    );
  }

  return (
    <div className={`${styles.expandableInputWrapper} ${styles.expandableInputExpanded}`}>
      <textarea
        ref={textareaRef}
        className={`input ${styles.expandableTextarea} ${className ?? ''}`}
        placeholder={placeholder}
        aria-label={ariaLabel}
        value={value}
        onChange={handleChange}
        disabled={disabled}
        rows={2}
      />
      <button
        type="button"
        className={styles.expandableToggle}
        disabled={disabled}
        onClick={() => setCollapsed(true)}
        title={t('common.collapse')}
        aria-label={t('common.collapse')}
      >
        ▲
      </button>
    </div>
  );
}

function getValidationMessage(
  t: ReturnType<typeof useTranslation>['t'],
  errorCode?: PayloadParamValidationErrorCode
) {
  if (!errorCode) return undefined;
  return t(`config_management.visual.validation.${errorCode}`);
}

function buildProtocolOptions(
  t: ReturnType<typeof useTranslation>['t'],
  rules: Array<{ models: PayloadModelEntry[] }>
) {
  const options: Array<{ value: string; label: string }> = VISUAL_CONFIG_PROTOCOL_OPTIONS.map(
    (option) => ({
      value: option.value,
      label: t(option.labelKey, { defaultValue: option.defaultLabel }),
    })
  );
  const seen = new Set<string>(options.map((option) => option.value));

  for (const rule of rules) {
    for (const model of rule.models) {
      const protocol = model.protocol;
      if (!protocol || !protocol.trim() || seen.has(protocol)) continue;
      seen.add(protocol);
      options.push({ value: protocol, label: protocol });
    }
  }

  return options;
}

type CredentialOption = {
  authIndex: string;
  title: string;
  subtitle: string;
};

type CredentialDescriptor = {
  authIndex: string;
  provider: string;
  primary: string;
  id?: string;
  secondary?: string;
  sourceKind: 'oauth' | 'provider_key';
};

function normalizeAuthIndex(value: unknown): string {
  return String(value ?? '').trim();
}

function isTruthyFlag(value: unknown): boolean {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    return normalized === 'true' || normalized === '1' || normalized === 'yes' || normalized === 'on';
  }
  return false;
}

function buildAuthFileCredentialDescriptors(files: AuthFileItem[]): CredentialDescriptor[] {
  const seen = new Set<string>();
  const options: CredentialDescriptor[] = [];

  files.forEach((file) => {
    const authIndex = normalizeAuthIndex(file.authIndex ?? file['auth_index']);
    if (!authIndex || seen.has(authIndex)) return;

    const runtimeOnly = isTruthyFlag(file.runtimeOnly ?? file.runtime_only);
    const disabled = isTruthyFlag(file.disabled);
    const status = String(file.status ?? '').trim().toLowerCase();
    const path = String(file.path ?? '').trim();
    const source = String(file.source ?? '').trim().toLowerCase();
    if (runtimeOnly || disabled || status === 'disabled') return;
    if (!path && source !== 'file') return;

    seen.add(authIndex);

    const provider = String(file.provider ?? file.type ?? 'unknown').trim() || 'unknown';
    const accountType = String(file['account_type'] ?? '').trim().toLowerCase();
    const rawAccount = String(file.account ?? file.email ?? '').trim();
    const authFileID = String(file.name ?? file.id ?? '').trim();
    const primary =
      accountType === 'api_key'
        ? (rawAccount ? maskApiKey(rawAccount) : '')
        : rawAccount || String(file.label ?? file.name ?? '').trim();

    options.push({
      authIndex,
      provider,
      id: authFileID,
      primary,
      sourceKind: 'oauth',
    });
  });

  return options;
}

async function sha256Hex(input: string): Promise<string> {
  const cryptoApi = globalThis.crypto;
  const data = new TextEncoder().encode(input);
  if (cryptoApi?.subtle) {
    const digest = await cryptoApi.subtle.digest('SHA-256', data);
    return Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, '0')).join('');
  }

  const K = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4,
    0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe,
    0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f,
    0x4a7484aa, 0x5cb0a9dc, 0x76f988da, 0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
    0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc,
    0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
    0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070, 0x19a4c116,
    0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7,
    0xc67178f2,
  ];
  const rotr = (value: number, bits: number) => (value >>> bits) | (value << (32 - bits));
  const paddedLength = Math.ceil((data.length + 9) / 64) * 64;
  const bytes = new Uint8Array(paddedLength);
  bytes.set(data);
  bytes[data.length] = 0x80;
  const bitLength = data.length * 8;
  const view = new DataView(bytes.buffer);
  view.setUint32(bytes.length - 8, Math.floor(bitLength / 0x100000000), false);
  view.setUint32(bytes.length - 4, bitLength >>> 0, false);

  let h0 = 0x6a09e667;
  let h1 = 0xbb67ae85;
  let h2 = 0x3c6ef372;
  let h3 = 0xa54ff53a;
  let h4 = 0x510e527f;
  let h5 = 0x9b05688c;
  let h6 = 0x1f83d9ab;
  let h7 = 0x5be0cd19;

  const w = new Uint32Array(64);
  for (let offset = 0; offset < bytes.length; offset += 64) {
    for (let index = 0; index < 16; index += 1) {
      w[index] = view.getUint32(offset + index * 4, false);
    }
    for (let index = 16; index < 64; index += 1) {
      const s0 = rotr(w[index - 15], 7) ^ rotr(w[index - 15], 18) ^ (w[index - 15] >>> 3);
      const s1 = rotr(w[index - 2], 17) ^ rotr(w[index - 2], 19) ^ (w[index - 2] >>> 10);
      w[index] = (w[index - 16] + s0 + w[index - 7] + s1) >>> 0;
    }

    let a = h0;
    let b = h1;
    let c = h2;
    let d = h3;
    let e = h4;
    let f = h5;
    let g = h6;
    let h = h7;

    for (let index = 0; index < 64; index += 1) {
      const s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
      const ch = (e & f) ^ (~e & g);
      const temp1 = (h + s1 + ch + K[index] + w[index]) >>> 0;
      const s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
      const maj = (a & b) ^ (a & c) ^ (b & c);
      const temp2 = (s0 + maj) >>> 0;

      h = g;
      g = f;
      f = e;
      e = (d + temp1) >>> 0;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) >>> 0;
    }

    h0 = (h0 + a) >>> 0;
    h1 = (h1 + b) >>> 0;
    h2 = (h2 + c) >>> 0;
    h3 = (h3 + d) >>> 0;
    h4 = (h4 + e) >>> 0;
    h5 = (h5 + f) >>> 0;
    h6 = (h6 + g) >>> 0;
    h7 = (h7 + h) >>> 0;
  }

  return [h0, h1, h2, h3, h4, h5, h6, h7]
    .map((value) => value.toString(16).padStart(8, '0'))
    .join('');
}

class StableIDGenerator {
  private readonly counters = new Map<string, number>();

  async next(kind: string, ...parts: string[]): Promise<{ id: string; token: string }> {
    const payload = kind + parts.map((part) => `\0${part.trim()}`).join('');
    let short = (await sha256Hex(payload)).slice(0, 12);
    if (!short) short = '000000000000';
    if (short.length < 12) short = short.padStart(12, '0');
    const counterKey = `${kind}:${short}`;
    const current = this.counters.get(counterKey) ?? 0;
    this.counters.set(counterKey, current + 1);
    const token = current > 0 ? `${short}-${current}` : short;
    return { id: `${kind}:${token}`, token };
  }
}

function buildConfigIndexSeed({
  providerKey,
  compatName,
  baseUrl,
  proxyUrl,
  apiKey,
  source,
}: {
  providerKey: string;
  compatName?: string;
  baseUrl?: string;
  proxyUrl?: string;
  apiKey?: string;
  source?: string;
}): string {
  const parts = [`provider=${providerKey.trim().toLowerCase()}`];
  const compat = (compatName ?? '').trim().toLowerCase();
  const base = (baseUrl ?? '').trim();
  const proxy = (proxyUrl ?? '').trim();
  const key = (apiKey ?? '').trim();
  const origin = (source ?? '').trim();

  if (compat) parts.push(`compat=${compat}`);
  if (base) parts.push(`base=${base}`);
  if (proxy) parts.push(`proxy=${proxy}`);
  if (key) parts.push(`api_key=${key}`);
  if (origin) parts.push(`source=${origin}`);

  return `config:${parts.join('\x00')}`;
}

async function deriveConfigAuthIndex(seed: string): Promise<string> {
  return (await sha256Hex(seed)).slice(0, 16);
}

async function createProviderCredentialDescriptor(input: {
  generator: StableIDGenerator;
  kind: string;
  tokenParts: string[];
  providerKey: string;
  providerLabel: string;
  compatName?: string;
  baseUrl?: string;
  proxyUrl?: string;
  apiKey?: string;
}): Promise<CredentialDescriptor | null> {
  const apiKey = (input.apiKey ?? '').trim();
  if (!apiKey) return null;

  const { token } = await input.generator.next(input.kind, ...input.tokenParts);
  const source = `config:${input.providerKey}[${token}]`;
  const authIndex = await deriveConfigAuthIndex(
    buildConfigIndexSeed({
      providerKey: input.providerKey,
      compatName: input.compatName,
      baseUrl: input.baseUrl,
      proxyUrl: input.proxyUrl,
      apiKey,
      source,
    })
  );
  if (!authIndex) return null;

  return {
    authIndex,
    provider: input.providerLabel.trim() || input.providerKey,
    primary: maskApiKey(apiKey),
    secondary: (input.baseUrl ?? '').trim(),
    sourceKind: 'provider_key',
  };
}

async function buildProviderCredentialDescriptors(config: Config | null | undefined): Promise<CredentialDescriptor[]> {
  if (!config) return [];

  const generator = new StableIDGenerator();
  const descriptors: CredentialDescriptor[] = [];

  for (const entry of config.geminiApiKeys ?? []) {
    const descriptor = await createProviderCredentialDescriptor({
      generator,
      kind: 'gemini:apikey',
      tokenParts: [entry.apiKey ?? '', entry.baseUrl ?? ''],
      providerKey: 'gemini',
      providerLabel: 'gemini',
      baseUrl: entry.baseUrl,
      proxyUrl: entry.proxyUrl,
      apiKey: entry.apiKey,
    });
    if (descriptor) descriptors.push(descriptor);
  }

  for (const entry of config.claudeApiKeys ?? []) {
    const descriptor = await createProviderCredentialDescriptor({
      generator,
      kind: 'claude:apikey',
      tokenParts: [entry.apiKey ?? '', entry.baseUrl ?? ''],
      providerKey: 'claude',
      providerLabel: 'claude',
      baseUrl: entry.baseUrl,
      proxyUrl: entry.proxyUrl,
      apiKey: entry.apiKey,
    });
    if (descriptor) descriptors.push(descriptor);
  }

  for (const entry of config.codexApiKeys ?? []) {
    const descriptor = await createProviderCredentialDescriptor({
      generator,
      kind: 'codex:apikey',
      tokenParts: [entry.apiKey ?? '', entry.baseUrl ?? ''],
      providerKey: 'codex',
      providerLabel: 'codex',
      baseUrl: entry.baseUrl,
      proxyUrl: entry.proxyUrl,
      apiKey: entry.apiKey,
    });
    if (descriptor) descriptors.push(descriptor);
  }

  for (const entry of config.vertexApiKeys ?? []) {
    const descriptor = await createProviderCredentialDescriptor({
      generator,
      kind: 'vertex:apikey',
      tokenParts: [entry.apiKey ?? '', entry.baseUrl ?? '', entry.proxyUrl ?? ''],
      providerKey: 'vertex-apikey',
      providerLabel: 'vertex',
      baseUrl: entry.baseUrl,
      proxyUrl: entry.proxyUrl,
      apiKey: entry.apiKey,
    });
    if (descriptor) descriptors.push(descriptor);
  }

  for (const provider of config.openaiCompatibility ?? []) {
    const providerName = (provider.name ?? '').trim();
    const providerKey = providerName.toLowerCase() || 'openai-compatibility';
    const providerLabel = providerName || providerKey;
    for (const entry of provider.apiKeyEntries ?? []) {
      const descriptor = await createProviderCredentialDescriptor({
        generator,
        kind: `openai-compatibility:${providerKey}`,
        tokenParts: [entry.apiKey ?? '', provider.baseUrl ?? '', entry.proxyUrl ?? ''],
        providerKey,
        providerLabel,
        compatName: providerName,
        baseUrl: provider.baseUrl,
        proxyUrl: entry.proxyUrl,
        apiKey: entry.apiKey,
      });
      if (descriptor) descriptors.push(descriptor);
    }
  }

  return descriptors;
}

function mergeCredentialDescriptors(...groups: CredentialDescriptor[][]): CredentialDescriptor[] {
  const seen = new Set<string>();
  const merged: CredentialDescriptor[] = [];

  groups.flat().forEach((descriptor) => {
    if (!descriptor.authIndex || seen.has(descriptor.authIndex)) return;
    seen.add(descriptor.authIndex);
    merged.push(descriptor);
  });

  return merged;
}

function buildCredentialOptions(
  descriptors: CredentialDescriptor[],
  t: ReturnType<typeof useTranslation>['t']
): CredentialOption[] {
  return descriptors
    .map((descriptor) => {
      const sourceText =
        descriptor.sourceKind === 'provider_key'
          ? t('config_management.visual.api_keys.credential_provider_key', {
              defaultValue: 'Provider key',
            })
          : t('config_management.visual.api_keys.credential_oauth', {
              defaultValue: 'OAuth credential',
            });

      return {
        authIndex: descriptor.authIndex,
        title:
          descriptor.sourceKind === 'oauth'
            ? [descriptor.provider, descriptor.id, descriptor.primary].filter(Boolean).join(' · ')
            : [descriptor.provider, descriptor.secondary, descriptor.primary].filter(Boolean).join(' · '),
        subtitle: `${sourceText} · auth_index: ${descriptor.authIndex}`,
      };
    })
    .sort((left, right) => left.title.localeCompare(right.title, undefined, { sensitivity: 'base' }));
}

export const ApiKeysCardEditor = memo(function ApiKeysCardEditor({
  value,
  disabled,
  onChange,
}: {
  value: VisualApiKeyEntry[];
  disabled?: boolean;
  onChange: (nextValue: VisualApiKeyEntry[]) => void;
}) {
  const { t } = useTranslation();
  const showNotification = useNotificationStore((state) => state.showNotification);
  const config = useConfigStore((state) => state.config);
  const fetchConfig = useConfigStore((state) => state.fetchConfig);
  const apiKeys = useMemo(
    () =>
      value.map((entry) => ({
        id: entry.id || makeClientId(),
        name: String(entry.name ?? '').trim(),
        key: String(entry.key ?? '').trim(),
        allowedAuthIndices: Array.isArray(entry.allowedAuthIndices)
          ? Array.from(new Set(entry.allowedAuthIndices.map((item) => String(item ?? '').trim()).filter(Boolean)))
          : [],
      })),
    [value]
  );

  const apiKeyInputId = useId();
  const apiKeyNameInputId = useId();
  const apiKeyHintId = `${apiKeyInputId}-hint`;
  const apiKeyErrorId = `${apiKeyInputId}-error`;
  const [modalOpen, setModalOpen] = useState(false);
  const [editingApiKeyId, setEditingApiKeyId] = useState<string | null>(null);
  const [nameInputValue, setNameInputValue] = useState('');
  const [inputValue, setInputValue] = useState('');
  const [selectedAuthIndices, setSelectedAuthIndices] = useState<string[]>([]);
  const [formError, setFormError] = useState('');
  const [credentialDescriptors, setCredentialDescriptors] = useState<CredentialDescriptor[]>([]);
  const [credentialLoading, setCredentialLoading] = useState(true);

  function generateSecureApiKey(): string {
    const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    const array = new Uint8Array(17);
    crypto.getRandomValues(array);
    return 'sk-' + Array.from(array, (b) => charset[b % charset.length]).join('');
  }

  const openAddModal = () => {
    setEditingApiKeyId(null);
    setNameInputValue('');
    setInputValue('');
    setSelectedAuthIndices([]);
    setFormError('');
    setModalOpen(true);
  };

  const openEditModal = (apiKeyId: string) => {
    const editingEntry = apiKeys.find((entry) => entry.id === apiKeyId);
    setEditingApiKeyId(apiKeyId);
    setNameInputValue(editingEntry?.name ?? '');
    setInputValue(editingEntry?.key ?? '');
    setSelectedAuthIndices(editingEntry?.allowedAuthIndices ?? []);
    setFormError('');
    setModalOpen(true);
  };

  const closeModal = () => {
    setModalOpen(false);
    setNameInputValue('');
    setInputValue('');
    setEditingApiKeyId(null);
    setSelectedAuthIndices([]);
    setFormError('');
  };

  useEffect(() => {
    let active = true;
    Promise.all([
      authFilesApi
        .list()
        .then((response) => buildAuthFileCredentialDescriptors(response.files ?? []))
        .catch(() => []),
      (config ? Promise.resolve(config) : fetchConfig().catch(() => null))
        .then((resolvedConfig) => buildProviderCredentialDescriptors(resolvedConfig as Config | null))
        .catch(() => []),
    ])
      .then(([authFileCredentials, providerCredentials]) => {
        if (!active) return;
        setCredentialDescriptors(mergeCredentialDescriptors(authFileCredentials, providerCredentials));
      })
      .finally(() => {
        if (!active) return;
        setCredentialLoading(false);
      });
    return () => {
      active = false;
    };
  }, [config, fetchConfig]);

  const handleDelete = (apiKeyId: string) => {
    onChange(apiKeys.filter((entry) => entry.id !== apiKeyId));
  };

  const handleSave = () => {
    const trimmed = inputValue.trim();
    if (!trimmed) {
      setFormError(t('config_management.visual.api_keys.error_empty'));
      return;
    }
    if (!isValidApiKeyCharset(trimmed)) {
      setFormError(t('config_management.visual.api_keys.error_invalid'));
      return;
    }

    const normalizedSelection = Array.from(
      new Set(selectedAuthIndices.map((item) => item.trim()).filter(Boolean))
    );
    const nextEntry: VisualApiKeyEntry = {
      id: editingApiKeyId ?? makeClientId(),
      name: nameInputValue.trim(),
      key: trimmed,
      allowedAuthIndices: normalizedSelection,
    };
    const nextKeys =
      editingApiKeyId === null
        ? [...apiKeys, nextEntry]
        : apiKeys.map((entry) => (entry.id === editingApiKeyId ? nextEntry : entry));
    onChange(nextKeys);
    closeModal();
  };

  const handleCopy = async (apiKey: string) => {
    const copied = await copyToClipboard(apiKey);
    showNotification(
      t(copied ? 'notification.link_copied' : 'notification.copy_failed'),
      copied ? 'success' : 'error'
    );
  };

  const handleGenerate = () => {
    setInputValue(generateSecureApiKey());
    setFormError('');
  };

  const credentialOptions = useMemo(
    () => buildCredentialOptions(credentialDescriptors, t),
    [credentialDescriptors, t]
  );
  const credentialOptionMap = useMemo(
    () => new Map(credentialOptions.map((option) => [option.authIndex, option])),
    [credentialOptions]
  );

  const toggleAuthIndex = (authIndex: string) => {
    setSelectedAuthIndices((current) =>
      current.includes(authIndex)
        ? current.filter((item) => item !== authIndex)
        : [...current, authIndex]
    );
  };

  const renderScopeSummary = (entry: VisualApiKeyEntry) => {
    if (!entry.allowedAuthIndices.length) {
      return t('config_management.visual.api_keys.scope_all', {
        defaultValue: 'All credentials',
      });
    }
    const labels = entry.allowedAuthIndices.map((authIndex) => {
      const option = credentialOptionMap.get(authIndex);
      return option
        ? option.title
        : t('config_management.visual.api_keys.credential_unknown', {
            authIndex,
            defaultValue: `Unknown credential (${authIndex})`,
          });
    });
    return labels.join(' / ');
  };

  const missingSelectedAuths = selectedAuthIndices.filter((authIndex) => !credentialOptionMap.has(authIndex));

  return (
    <div className="form-group" style={{ marginBottom: 0 }}>
      <div className={styles.blockHeaderRow}>
        <label style={{ margin: 0 }}>{t('config_management.visual.api_keys.label')}</label>
        <Button size="sm" onClick={openAddModal} disabled={disabled}>
          {t('config_management.visual.api_keys.add')}
        </Button>
      </div>

      {apiKeys.length === 0 ? (
        <div className={styles.emptyState}>{t('config_management.visual.api_keys.empty')}</div>
      ) : (
        <div className="item-list" style={{ marginTop: 4 }}>
          {apiKeys.map((entry, index) => (
            <div key={entry.id} className="item-row">
              <div className="item-meta">
                <div className="pill">#{index + 1}</div>
                <div className="item-title">
                  {entry.name || t('config_management.visual.api_keys.input_label')}
                </div>
                <div className="item-subtitle">{maskApiKey(String(entry.key || ''))}</div>
                <div className="item-subtitle">{renderScopeSummary(entry)}</div>
              </div>
              <div className="item-actions">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => handleCopy(entry.key)}
                  disabled={disabled}
                >
                  {t('common.copy')}
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => openEditModal(entry.id)}
                  disabled={disabled}
                >
                  {t('config_management.visual.common.edit')}
                </Button>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => handleDelete(entry.id)}
                  disabled={disabled}
                >
                  {t('config_management.visual.common.delete')}
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="hint">{t('config_management.visual.api_keys.hint')}</div>

      <Modal
        open={modalOpen}
        onClose={closeModal}
        title={
          editingApiKeyId !== null
            ? t('config_management.visual.api_keys.edit_title')
            : t('config_management.visual.api_keys.add_title')
        }
        footer={
          <>
            <Button variant="secondary" onClick={closeModal} disabled={disabled}>
              {t('config_management.visual.common.cancel')}
            </Button>
            <Button onClick={handleSave} disabled={disabled}>
              {editingApiKeyId !== null
                ? t('config_management.visual.common.update')
                : t('config_management.visual.common.add')}
            </Button>
          </>
        }
      >
        <div className="form-group">
          <label htmlFor={apiKeyNameInputId}>
            {t('config_management.visual.api_keys.name_label')}
          </label>
          <input
            id={apiKeyNameInputId}
            className="input"
            placeholder={t('config_management.visual.api_keys.name_placeholder')}
            value={nameInputValue}
            onChange={(e) => setNameInputValue(e.target.value)}
            disabled={disabled}
          />
          <div className="hint">{t('config_management.visual.api_keys.name_hint')}</div>
        </div>

        <div className="form-group">
          <label htmlFor={apiKeyInputId}>
            {t('config_management.visual.api_keys.input_label')}
          </label>
          <div className={styles.apiKeyModalInputRow}>
            <input
              id={apiKeyInputId}
              className="input"
              placeholder={t('config_management.visual.api_keys.input_placeholder')}
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              disabled={disabled}
              aria-describedby={formError ? `${apiKeyErrorId} ${apiKeyHintId}` : apiKeyHintId}
              aria-invalid={Boolean(formError)}
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={handleGenerate}
              disabled={disabled}
            >
              {t('config_management.visual.api_keys.generate')}
            </Button>
          </div>
          <div id={apiKeyHintId} className="hint">
            {t('config_management.visual.api_keys.input_hint')}
          </div>
          {formError && (
            <div id={apiKeyErrorId} className="error-box">
              {formError}
            </div>
          )}
        </div>

        <div className="form-group">
          <div className={styles.subsectionHeader}>
            <div className={styles.subsectionTitle}>
              {t('config_management.visual.api_keys.scope_label', {
                defaultValue: 'Allowed credentials',
              })}
            </div>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setSelectedAuthIndices([])}
              disabled={disabled}
            >
              {t('config_management.visual.api_keys.scope_reset', {
                defaultValue: 'Use all',
              })}
            </Button>
          </div>
          <div className="hint">
            {t('config_management.visual.api_keys.scope_hint', {
              defaultValue:
                'Leave empty to allow all provider API keys and OAuth credentials. Select entries below to restrict this client key.',
            })}
          </div>

          {credentialLoading ? (
            <div className={styles.emptyState}>
              {t('config_management.visual.api_keys.credentials_loading', {
                defaultValue: 'Loading credentials…',
              })}
            </div>
          ) : credentialOptions.length === 0 ? (
            <div className={styles.emptyState}>
              {t('config_management.visual.api_keys.credentials_empty', {
                defaultValue: 'No provider keys or OAuth credentials available yet.',
              })}
            </div>
          ) : (
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 8,
                maxHeight: 280,
                overflow: 'auto',
                marginTop: 8,
              }}
            >
              {credentialOptions.map((option) => {
                const checked = selectedAuthIndices.includes(option.authIndex);
                return (
                  <label
                    key={option.authIndex}
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 10,
                      padding: '10px 12px',
                      borderRadius: 12,
                      border: '1px solid var(--border-color)',
                      background: 'var(--bg-secondary)',
                      cursor: disabled ? 'default' : 'pointer',
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleAuthIndex(option.authIndex)}
                      disabled={disabled}
                    />
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontWeight: 600 }}>{option.title}</div>
                      <div className="hint">{option.subtitle}</div>
                    </div>
                  </label>
                );
              })}
            </div>
          )}

          {missingSelectedAuths.length > 0 && (
            <div className="hint" style={{ marginTop: 8 }}>
              {t('config_management.visual.api_keys.credentials_missing', {
                defaultValue: 'Selected auth_index values not found in current credential list:',
              })}{' '}
              {missingSelectedAuths.join(', ')}
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
});

const StringListEditor = memo(function StringListEditor({
  value,
  disabled,
  placeholder,
  inputAriaLabel,
  onChange,
}: {
  value: string[];
  disabled?: boolean;
  placeholder?: string;
  inputAriaLabel?: string;
  onChange: (next: string[]) => void;
}) {
  const { t } = useTranslation();
  const items = value.length ? value : [];
  const [itemIds, setItemIds] = useState(() => items.map(() => makeClientId()));
  const renderItemIds = useMemo(() => {
    if (itemIds.length === items.length) return itemIds;
    if (itemIds.length > items.length) return itemIds.slice(0, items.length);
    return [
      ...itemIds,
      ...Array.from({ length: items.length - itemIds.length }, () => makeClientId()),
    ];
  }, [itemIds, items.length]);

  const updateItem = (index: number, nextValue: string) =>
    onChange(items.map((item, i) => (i === index ? nextValue : item)));
  const addItem = () => {
    setItemIds([...renderItemIds, makeClientId()]);
    onChange([...items, '']);
  };
  const removeItem = (index: number) => {
    setItemIds(renderItemIds.filter((_, i) => i !== index));
    onChange(items.filter((_, i) => i !== index));
  };

  return (
    <div className={styles.stringList}>
      {items.map((item, index) => (
        <div key={renderItemIds[index] ?? `item-${index}`} className={styles.stringListRow}>
          <ExpandableInput
            placeholder={placeholder}
            ariaLabel={inputAriaLabel ?? placeholder}
            value={item}
            onChange={(nextValue) => updateItem(index, nextValue)}
            disabled={disabled}
          />
          <Button variant="ghost" size="sm" onClick={() => removeItem(index)} disabled={disabled}>
            {t('config_management.visual.common.delete')}
          </Button>
        </div>
      ))}
      <div className={styles.actionRow}>
        <Button variant="secondary" size="sm" onClick={addItem} disabled={disabled}>
          {t('config_management.visual.common.add')}
        </Button>
      </div>
    </div>
  );
});

export const PayloadRulesEditor = memo(function PayloadRulesEditor({
  value,
  disabled,
  protocolFirst = false,
  rawJsonValues = false,
  onChange,
}: {
  value: PayloadRule[];
  disabled?: boolean;
  protocolFirst?: boolean;
  rawJsonValues?: boolean;
  onChange: (next: PayloadRule[]) => void;
}) {
  const { t } = useTranslation();
  const rules = value;
  const protocolOptions = useMemo(() => buildProtocolOptions(t, rules), [rules, t]);
  const payloadValueTypeOptions = useMemo(
    () =>
      VISUAL_CONFIG_PAYLOAD_VALUE_TYPE_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.labelKey, { defaultValue: option.defaultLabel }),
      })),
    [t]
  );
  const booleanValueOptions = useMemo(
    () => [
      { value: 'true', label: t('config_management.visual.payload_rules.boolean_true') },
      { value: 'false', label: t('config_management.visual.payload_rules.boolean_false') },
    ],
    [t]
  );

  const addRule = () => onChange([...rules, { id: makeClientId(), models: [], params: [] }]);
  const removeRule = (ruleIndex: number) => onChange(rules.filter((_, i) => i !== ruleIndex));

  const updateRule = (ruleIndex: number, patch: Partial<PayloadRule>) =>
    onChange(rules.map((rule, i) => (i === ruleIndex ? { ...rule, ...patch } : rule)));

  const addModel = (ruleIndex: number) => {
    const rule = rules[ruleIndex];
    const nextModel: PayloadModelEntry = { id: makeClientId(), name: '', protocol: undefined };
    updateRule(ruleIndex, { models: [...rule.models, nextModel] });
  };

  const removeModel = (ruleIndex: number, modelIndex: number) => {
    const rule = rules[ruleIndex];
    updateRule(ruleIndex, { models: rule.models.filter((_, i) => i !== modelIndex) });
  };

  const updateModel = (
    ruleIndex: number,
    modelIndex: number,
    patch: Partial<PayloadModelEntry>
  ) => {
    const rule = rules[ruleIndex];
    updateRule(ruleIndex, {
      models: rule.models.map((m, i) => (i === modelIndex ? { ...m, ...patch } : m)),
    });
  };

  const addParam = (ruleIndex: number) => {
    const rule = rules[ruleIndex];
    const nextParam: PayloadParamEntry = {
      id: makeClientId(),
      path: '',
      valueType: rawJsonValues ? 'json' : 'string',
      value: '',
    };
    updateRule(ruleIndex, { params: [...rule.params, nextParam] });
  };

  const removeParam = (ruleIndex: number, paramIndex: number) => {
    const rule = rules[ruleIndex];
    updateRule(ruleIndex, { params: rule.params.filter((_, i) => i !== paramIndex) });
  };

  const updateParam = (
    ruleIndex: number,
    paramIndex: number,
    patch: Partial<PayloadParamEntry>
  ) => {
    const rule = rules[ruleIndex];
    updateRule(ruleIndex, {
      params: rule.params.map((p, i) => (i === paramIndex ? { ...p, ...patch } : p)),
    });
  };

  const getValuePlaceholder = (valueType: PayloadParamValueType) => {
    switch (valueType) {
      case 'string':
        return t('config_management.visual.payload_rules.value_string');
      case 'number':
        return t('config_management.visual.payload_rules.value_number');
      case 'boolean':
        return t('config_management.visual.payload_rules.value_boolean');
      case 'json':
        return t('config_management.visual.payload_rules.value_json');
      default:
        return t('config_management.visual.payload_rules.value_default');
    }
  };

  const getParamErrorMessage = (param: PayloadParamEntry) => {
    const errorCode = getPayloadParamValidationError(
      rawJsonValues ? { ...param, valueType: 'json' } : param
    );
    return getValidationMessage(t, errorCode);
  };

  const renderParamValueEditor = (
    ruleIndex: number,
    paramIndex: number,
    param: PayloadParamEntry
  ) => {
    if (rawJsonValues) {
      return (
        <textarea
          className={`input ${styles.payloadJsonInput}`}
          placeholder={t('config_management.visual.payload_rules.value_raw_json')}
          aria-label={t('config_management.visual.payload_rules.param_value')}
          value={param.value}
          onChange={(e) =>
            updateParam(ruleIndex, paramIndex, { value: e.target.value, valueType: 'json' })
          }
          disabled={disabled}
        />
      );
    }

    if (param.valueType === 'boolean') {
      return (
        <Select
          value={
            param.value.toLowerCase() === 'true' || param.value.toLowerCase() === 'false'
              ? param.value.toLowerCase()
              : ''
          }
          options={booleanValueOptions}
          placeholder={t('config_management.visual.payload_rules.value_boolean')}
          disabled={disabled}
          ariaLabel={t('config_management.visual.payload_rules.param_value')}
          onChange={(nextValue) => updateParam(ruleIndex, paramIndex, { value: nextValue })}
        />
      );
    }

    if (param.valueType === 'json') {
      return (
        <textarea
          className={`input ${styles.payloadJsonInput}`}
          placeholder={getValuePlaceholder(param.valueType)}
          aria-label={t('config_management.visual.payload_rules.param_value')}
          value={param.value}
          onChange={(e) => updateParam(ruleIndex, paramIndex, { value: e.target.value })}
          disabled={disabled}
        />
      );
    }

    return (
      <ExpandableInput
        placeholder={getValuePlaceholder(param.valueType)}
        ariaLabel={t('config_management.visual.payload_rules.param_value')}
        value={param.value}
        onChange={(nextValue) => updateParam(ruleIndex, paramIndex, { value: nextValue })}
        disabled={disabled}
      />
    );
  };

  return (
    <div className={styles.blockStack}>
      {rules.map((rule, ruleIndex) => (
        <div key={rule.id} className={styles.ruleCard}>
          <div className={styles.ruleCardHeader}>
            <div className={styles.ruleCardTitle}>
              {t('config_management.visual.payload_rules.rule')} {ruleIndex + 1}
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => removeRule(ruleIndex)}
              disabled={disabled}
            >
              {t('config_management.visual.common.delete')}
            </Button>
          </div>

          <div className={styles.blockStack}>
            <div className={styles.blockLabel}>
              {t('config_management.visual.payload_rules.models')}
            </div>
            {(rule.models.length ? rule.models : []).map((model, modelIndex) => (
              <div
                key={model.id}
                className={[
                  styles.payloadRuleModelRow,
                  protocolFirst ? styles.payloadRuleModelRowProtocolFirst : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
              >
                {protocolFirst ? (
                  <>
                    <Select
                      value={model.protocol ?? ''}
                      options={protocolOptions}
                      disabled={disabled}
                      ariaLabel={t('config_management.visual.payload_rules.provider_type')}
                      onChange={(nextValue) =>
                        updateModel(ruleIndex, modelIndex, {
                          protocol: (nextValue || undefined) as PayloadModelEntry['protocol'],
                        })
                      }
                    />
                    <ExpandableInput
                      placeholder={t('config_management.visual.payload_rules.model_name')}
                      ariaLabel={t('config_management.visual.payload_rules.model_name')}
                      value={model.name}
                      onChange={(nextValue) => updateModel(ruleIndex, modelIndex, { name: nextValue })}
                      disabled={disabled}
                    />
                  </>
                ) : (
                  <>
                    <ExpandableInput
                      placeholder={t('config_management.visual.payload_rules.model_name')}
                      ariaLabel={t('config_management.visual.payload_rules.model_name')}
                      value={model.name}
                      onChange={(nextValue) => updateModel(ruleIndex, modelIndex, { name: nextValue })}
                      disabled={disabled}
                    />
                    <Select
                      value={model.protocol ?? ''}
                      options={protocolOptions}
                      disabled={disabled}
                      ariaLabel={t('config_management.visual.payload_rules.provider_type')}
                      onChange={(nextValue) =>
                        updateModel(ruleIndex, modelIndex, {
                          protocol: (nextValue || undefined) as PayloadModelEntry['protocol'],
                        })
                      }
                    />
                  </>
                )}
                <Button
                  variant="ghost"
                  size="sm"
                  className={styles.payloadRowActionButton}
                  onClick={() => removeModel(ruleIndex, modelIndex)}
                  disabled={disabled}
                >
                  {t('config_management.visual.common.delete')}
                </Button>
              </div>
            ))}
            <div className={styles.actionRow}>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => addModel(ruleIndex)}
                disabled={disabled}
              >
                {t('config_management.visual.payload_rules.add_model')}
              </Button>
            </div>
          </div>

          <div className={styles.blockStack}>
            <div className={styles.blockLabel}>
              {t('config_management.visual.payload_rules.params')}
            </div>
            {(rule.params.length ? rule.params : []).map((param, paramIndex) => {
              const paramError = getParamErrorMessage(param);

              return (
                <div key={param.id} className={styles.payloadRuleParamGroup}>
                  <div className={styles.payloadRuleParamRow}>
                    <ExpandableInput
                      placeholder={t('config_management.visual.payload_rules.json_path')}
                      ariaLabel={t('config_management.visual.payload_rules.json_path')}
                      value={param.path}
                      onChange={(nextValue) => updateParam(ruleIndex, paramIndex, { path: nextValue })}
                      disabled={disabled}
                    />
                    {rawJsonValues ? null : (
                      <Select
                        value={param.valueType}
                        options={payloadValueTypeOptions}
                        disabled={disabled}
                        ariaLabel={t('config_management.visual.payload_rules.param_type')}
                        onChange={(nextValue) =>
                          updateParam(ruleIndex, paramIndex, {
                            valueType: nextValue as PayloadParamValueType,
                            value:
                              nextValue === 'boolean'
                                ? 'true'
                                : nextValue === 'json' && param.value.trim() === ''
                                  ? '{}'
                                  : param.value,
                          })
                        }
                      />
                    )}
                    {renderParamValueEditor(ruleIndex, paramIndex, param)}
                    <Button
                      variant="ghost"
                      size="sm"
                      className={styles.payloadRowActionButton}
                      onClick={() => removeParam(ruleIndex, paramIndex)}
                      disabled={disabled}
                    >
                      {t('config_management.visual.common.delete')}
                    </Button>
                  </div>
                  {paramError && (
                    <div className={`error-box ${styles.payloadParamError}`}>{paramError}</div>
                  )}
                </div>
              );
            })}
            <div className={styles.actionRow}>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => addParam(ruleIndex)}
                disabled={disabled}
              >
                {t('config_management.visual.payload_rules.add_param')}
              </Button>
            </div>
          </div>
        </div>
      ))}

      {rules.length === 0 && (
        <div className={styles.emptyState}>
          {t('config_management.visual.payload_rules.no_rules')}
        </div>
      )}

      <div className={styles.actionRow}>
        <Button variant="secondary" size="sm" onClick={addRule} disabled={disabled}>
          {t('config_management.visual.payload_rules.add_rule')}
        </Button>
      </div>
    </div>
  );
});

export const PayloadFilterRulesEditor = memo(function PayloadFilterRulesEditor({
  value,
  disabled,
  onChange,
}: {
  value: PayloadFilterRule[];
  disabled?: boolean;
  onChange: (next: PayloadFilterRule[]) => void;
}) {
  const { t } = useTranslation();
  const rules = value;
  const protocolOptions = useMemo(() => buildProtocolOptions(t, rules), [rules, t]);

  const addRule = () => onChange([...rules, { id: makeClientId(), models: [], params: [] }]);
  const removeRule = (ruleIndex: number) => onChange(rules.filter((_, i) => i !== ruleIndex));

  const updateRule = (ruleIndex: number, patch: Partial<PayloadFilterRule>) =>
    onChange(rules.map((rule, i) => (i === ruleIndex ? { ...rule, ...patch } : rule)));

  const addModel = (ruleIndex: number) => {
    const rule = rules[ruleIndex];
    const nextModel: PayloadModelEntry = { id: makeClientId(), name: '', protocol: undefined };
    updateRule(ruleIndex, { models: [...rule.models, nextModel] });
  };

  const removeModel = (ruleIndex: number, modelIndex: number) => {
    const rule = rules[ruleIndex];
    updateRule(ruleIndex, { models: rule.models.filter((_, i) => i !== modelIndex) });
  };

  const updateModel = (
    ruleIndex: number,
    modelIndex: number,
    patch: Partial<PayloadModelEntry>
  ) => {
    const rule = rules[ruleIndex];
    updateRule(ruleIndex, {
      models: rule.models.map((m, i) => (i === modelIndex ? { ...m, ...patch } : m)),
    });
  };

  return (
    <div className={styles.blockStack}>
      {rules.map((rule, ruleIndex) => (
        <div key={rule.id} className={styles.ruleCard}>
          <div className={styles.ruleCardHeader}>
            <div className={styles.ruleCardTitle}>
              {t('config_management.visual.payload_rules.rule')} {ruleIndex + 1}
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => removeRule(ruleIndex)}
              disabled={disabled}
            >
              {t('config_management.visual.common.delete')}
            </Button>
          </div>

          <div className={styles.blockStack}>
            <div className={styles.blockLabel}>
              {t('config_management.visual.payload_rules.models')}
            </div>
            {rule.models.map((model, modelIndex) => (
              <div key={model.id} className={styles.payloadFilterModelRow}>
                <ExpandableInput
                  placeholder={t('config_management.visual.payload_rules.model_name')}
                  ariaLabel={t('config_management.visual.payload_rules.model_name')}
                  value={model.name}
                  onChange={(nextValue) => updateModel(ruleIndex, modelIndex, { name: nextValue })}
                  disabled={disabled}
                />
                <Select
                  value={model.protocol ?? ''}
                  options={protocolOptions}
                  disabled={disabled}
                  ariaLabel={t('config_management.visual.payload_rules.provider_type')}
                  onChange={(nextValue) =>
                    updateModel(ruleIndex, modelIndex, {
                      protocol: (nextValue || undefined) as PayloadModelEntry['protocol'],
                    })
                  }
                />
                <Button
                  variant="ghost"
                  size="sm"
                  className={styles.payloadRowActionButton}
                  onClick={() => removeModel(ruleIndex, modelIndex)}
                  disabled={disabled}
                >
                  {t('config_management.visual.common.delete')}
                </Button>
              </div>
            ))}
            <div className={styles.actionRow}>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => addModel(ruleIndex)}
                disabled={disabled}
              >
                {t('config_management.visual.payload_rules.add_model')}
              </Button>
            </div>
          </div>

          <div className={styles.blockStack}>
            <div className={styles.blockLabel}>
              {t('config_management.visual.payload_rules.remove_params')}
            </div>
            <StringListEditor
              value={rule.params}
              disabled={disabled}
              placeholder={t('config_management.visual.payload_rules.json_path_filter')}
              inputAriaLabel={t('config_management.visual.payload_rules.json_path_filter')}
              onChange={(params) => updateRule(ruleIndex, { params })}
            />
          </div>
        </div>
      ))}

      {rules.length === 0 && (
        <div className={styles.emptyState}>
          {t('config_management.visual.payload_rules.no_rules')}
        </div>
      )}

      <div className={styles.actionRow}>
        <Button variant="secondary" size="sm" onClick={addRule} disabled={disabled}>
          {t('config_management.visual.payload_rules.add_rule')}
        </Button>
      </div>
    </div>
  );
});
