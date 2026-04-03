import { useCallback, useEffect, useMemo, useState } from 'react';
import { parseDocument } from 'yaml';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import {
  ApiKeyEditorModal,
  useCredentialOptions,
} from '@/components/config/VisualConfigEditorBlocks';
import { DiffModal } from '@/components/config/DiffModal';
import { applyClientAuthChangesToYaml } from '@/features/clientAuth/yaml';
import { useVisualConfig } from '@/hooks/useVisualConfig';
import { configFileApi } from '@/services/api/configFile';
import { useAuthStore, useConfigStore, useNotificationStore } from '@/stores';
import { copyToClipboard } from '@/utils/clipboard';
import { maskApiKey } from '@/utils/format';
import {
  ClientAuthScopeFilter,
  ClientAuthSort,
  isRestrictedClientKey,
  matchesClientAuthSearch,
} from '@/features/clientAuth/utils';
import styles from './ClientAuthPage.module.scss';

function buildScopeSummary(
  allowedAuthIndices: string[],
  credentialOptionMap: Map<string, { title: string }>,
  t: ReturnType<typeof useTranslation>['t']
): string {
  if (allowedAuthIndices.length === 0) {
    return t('client_auth.scope_all', { defaultValue: 'All credentials' });
  }

  const preview = allowedAuthIndices
    .slice(0, 3)
    .map((authIndex) => credentialOptionMap.get(authIndex)?.title ?? authIndex)
    .join(', ');
  const remainder = allowedAuthIndices.length > 3 ? ` +${allowedAuthIndices.length - 3}` : '';

  return t('client_auth.scope_limited', {
    defaultValue: '{{count}} credentials · {{preview}}{{remainder}}',
    count: allowedAuthIndices.length,
    preview,
    remainder,
  });
}

export function ClientAuthPage() {
  const { t } = useTranslation();
  const showNotification = useNotificationStore((state) => state.showNotification);
  const showConfirmation = useNotificationStore((state) => state.showConfirmation);
  const connectionStatus = useAuthStore((state) => state.connectionStatus);
  const disableControls = connectionStatus !== 'connected';

  const {
    visualValues,
    visualDirty,
    visualParseError,
    loadVisualValuesFromYaml,
    setVisualValues,
  } = useVisualConfig({ includeClientAuth: true });
  const { credentialOptionMap } = useCredentialOptions();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [serverYaml, setServerYaml] = useState('');
  const [mergedYaml, setMergedYaml] = useState('');
  const [diffModalOpen, setDiffModalOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [scopeFilter, setScopeFilter] = useState<ClientAuthScopeFilter>('all');
  const [sortMode, setSortMode] = useState<ClientAuthSort>('default');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingKeyId, setEditingKeyId] = useState<string | null>(null);
  const [editorSessionKey, setEditorSessionKey] = useState(0);

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await configFileApi.fetchConfigYaml();
      setServerYaml(data);
      setMergedYaml(data);
      loadVisualValuesFromYaml(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('notification.refresh_failed'));
    } finally {
      setLoading(false);
    }
  }, [loadVisualValuesFromYaml, t]);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const handleReload = useCallback(() => {
    if (!visualDirty) {
      void loadConfig();
      return;
    }

    showConfirmation({
      title: t('common.unsaved_changes_title'),
      message: t('config_management.reload_confirm_message'),
      confirmText: t('config_management.reload'),
      cancelText: t('common.cancel'),
      variant: 'danger',
      onConfirm: async () => {
        await loadConfig();
      },
    });
  }, [loadConfig, showConfirmation, t, visualDirty]);

  const handleConfirmSave = async () => {
    setSaving(true);
    try {
      await configFileApi.saveConfigYaml(mergedYaml);
      const latestContent = await configFileApi.fetchConfigYaml();
      setServerYaml(latestContent);
      setMergedYaml(latestContent);
      setDiffModalOpen(false);
      loadVisualValuesFromYaml(latestContent);

      try {
        useConfigStore.getState().clearCache();
        await useConfigStore.getState().fetchConfig(undefined, true);
      } catch (refreshError: unknown) {
        const message =
          refreshError instanceof Error
            ? refreshError.message
            : typeof refreshError === 'string'
              ? refreshError
              : '';
        showNotification(
          `${t('notification.refresh_failed')}${message ? `: ${message}` : ''}`,
          'error'
        );
      }

      showNotification(t('config_management.save_success'), 'success');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '';
      showNotification(`${t('notification.save_failed')}: ${message}`, 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (visualParseError) {
      showNotification(
        t('config_management.visual_mode_unavailable_detail', { message: visualParseError }),
        'error'
      );
      return;
    }

    setSaving(true);
    try {
      const latestServerYaml = await configFileApi.fetchConfigYaml();
      const latestDocument = parseDocument(latestServerYaml);
      if (latestDocument.errors.length > 0) {
        showNotification(
          t('config_management.visual_mode_latest_yaml_invalid', {
            message:
              latestDocument.errors[0]?.message ??
              t('config_management.visual_mode_save_blocked'),
          }),
          'error'
        );
        return;
      }

      const nextMergedYaml = applyClientAuthChangesToYaml(
        latestServerYaml,
        visualValues.authDir,
        visualValues.apiKeys
      );

      let diffOriginal = latestServerYaml;
      try {
        diffOriginal = latestDocument.toString({
          indent: 2,
          lineWidth: 120,
          minContentWidth: 0,
        });
      } catch {
        /* keep raw on parse failure */
      }

      if (diffOriginal === nextMergedYaml) {
        setServerYaml(latestServerYaml);
        setMergedYaml(nextMergedYaml);
        loadVisualValuesFromYaml(latestServerYaml);
        showNotification(t('config_management.diff.no_changes'), 'info');
        return;
      }

      setServerYaml(diffOriginal);
      setMergedYaml(nextMergedYaml);
      setDiffModalOpen(true);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '';
      showNotification(`${t('notification.save_failed')}: ${message}`, 'error');
    } finally {
      setSaving(false);
    }
  };

  const statusText = useMemo(() => {
    if (disableControls) return t('config_management.status_disconnected');
    if (loading) return t('config_management.status_loading');
    if (error) return t('config_management.status_load_failed');
    if (visualParseError) return t('config_management.visual_mode_unavailable');
    if (saving) return t('config_management.status_saving');
    if (visualDirty) return t('config_management.status_dirty');
    return t('config_management.status_loaded');
  }, [disableControls, error, loading, saving, t, visualDirty, visualParseError]);

  const statusClassName = [
    styles.statusBadge,
    error || visualParseError ? styles.statusError : '',
    visualDirty ? styles.statusDirty : '',
  ]
    .filter(Boolean)
    .join(' ');

  const editingEntry = useMemo(
    () => visualValues.apiKeys.find((entry) => entry.id === editingKeyId) ?? null,
    [editingKeyId, visualValues.apiKeys]
  );

  const visibleKeys = useMemo(() => {
    const filtered = visualValues.apiKeys.filter((entry) => {
      if (!matchesClientAuthSearch(entry, search)) return false;
      if (scopeFilter === 'restricted') return isRestrictedClientKey(entry);
      if (scopeFilter === 'unrestricted') return !isRestrictedClientKey(entry);
      return true;
    });

    if (sortMode === 'name') {
      return [...filtered].sort((left, right) =>
        (left.name || left.key).localeCompare(right.name || right.key, undefined, {
          sensitivity: 'base',
        })
      );
    }

    return filtered;
  }, [scopeFilter, search, sortMode, visualValues.apiKeys]);

  const stats = useMemo(() => {
    const total = visualValues.apiKeys.length;
    const restricted = visualValues.apiKeys.filter(isRestrictedClientKey).length;

    return {
      total,
      restricted,
      unrestricted: total - restricted,
    };
  }, [visualValues.apiKeys]);

  const scopeFilterOptions = useMemo(
    () => [
      {
        value: 'all',
        label: t('client_auth.filter_all', { defaultValue: 'All keys' }),
      },
      {
        value: 'restricted',
        label: t('client_auth.filter_restricted', { defaultValue: 'Restricted' }),
      },
      {
        value: 'unrestricted',
        label: t('client_auth.filter_unrestricted', { defaultValue: 'Unrestricted' }),
      },
    ],
    [t]
  );

  const sortOptions = useMemo(
    () => [
      {
        value: 'default',
        label: t('client_auth.sort_default', { defaultValue: 'Default order' }),
      },
      {
        value: 'name',
        label: t('client_auth.sort_name', { defaultValue: 'Name' }),
      },
    ],
    [t]
  );

  const openAddEditor = () => {
    setEditingKeyId(null);
    setEditorSessionKey((current) => current + 1);
    setEditorOpen(true);
  };

  const openEditEditor = (id: string) => {
    setEditingKeyId(id);
    setEditorSessionKey((current) => current + 1);
    setEditorOpen(true);
  };

  const closeEditor = () => {
    setEditorOpen(false);
    setEditingKeyId(null);
  };

  const handleSaveEntry = (nextEntry: (typeof visualValues.apiKeys)[number]) => {
    const nextKeys = editingKeyId
      ? visualValues.apiKeys.map((entry) => (entry.id === editingKeyId ? nextEntry : entry))
      : [...visualValues.apiKeys, nextEntry];
    setVisualValues({ apiKeys: nextKeys });
  };

  const handleDeleteEntry = (id: string) => {
    setVisualValues({
      apiKeys: visualValues.apiKeys.filter((entry) => entry.id !== id),
    });
  };

  const handleCopyEntry = async (key: string) => {
    const copied = await copyToClipboard(key);
    showNotification(
      t(copied ? 'notification.link_copied' : 'notification.copy_failed'),
      copied ? 'success' : 'error'
    );
  };

  return (
    <div className={styles.container}>
      <div className={styles.pageHeader}>
        <div className={styles.pageHeaderCopy}>
          <span className={styles.pageEyebrow}>
            {t('client_auth.eyebrow', { defaultValue: 'Client Access' })}
          </span>
          <h1 className={styles.pageTitle}>{t('client_auth.title')}</h1>
          <p className={styles.description}>{t('client_auth.description')}</p>
        </div>

        <div className={styles.pageMeta}>
          <div className={statusClassName}>{statusText}</div>
          <div className={styles.headerActions}>
            <Button variant="secondary" onClick={handleReload} disabled={loading || saving}>
              {t('config_management.reload')}
            </Button>
            <Button
              variant="secondary"
              onClick={openAddEditor}
              disabled={disableControls || loading || saving}
            >
              {t('client_auth.add_key', { defaultValue: 'Add client key' })}
            </Button>
            <Button
              onClick={handleSave}
              disabled={disableControls || loading || saving || !visualDirty || !!visualParseError}
            >
              {t('config_management.save')}
            </Button>
          </div>
        </div>
      </div>

      <div className={styles.workspaceShell}>
        {error ? <div className="error-box">{error}</div> : null}
        {!error && visualParseError ? (
          <div className="error-box">
            {t('config_management.visual_mode_unavailable_detail', { message: visualParseError })}
          </div>
        ) : null}

        <div className={styles.workspace}>
          <aside className={styles.summaryRail}>
            <Card
              className={styles.summaryCard}
              title={t('client_auth.auth_dir', { defaultValue: 'Client auth directory' })}
            >
              <Input
                label={t('client_auth.auth_dir', { defaultValue: 'Client auth directory' })}
                hint={t('client_auth.auth_dir_hint', {
                  defaultValue: 'Controls where local client authentication artifacts are stored.',
                })}
                value={visualValues.authDir}
                onChange={(event) => setVisualValues({ authDir: event.target.value })}
                disabled={disableControls || loading}
                placeholder="~/.cli-proxy-api"
              />
              <p className={styles.helpText}>
                {t('client_auth.auth_dir_help', {
                  defaultValue:
                    'This page only manages client authentication. Remote-management settings stay on the config panel.',
                })}
              </p>
            </Card>

            <Card
              className={styles.summaryCard}
              title={t('client_auth.summary_title', { defaultValue: 'Key overview' })}
            >
              <div className={styles.statList}>
                <div className={styles.statItem}>
                  <span className={styles.statLabel}>
                    {t('client_auth.total_keys', { defaultValue: 'Total keys' })}
                  </span>
                  <strong className={styles.statValue}>{stats.total}</strong>
                </div>
                <div className={styles.statItem}>
                  <span className={styles.statLabel}>
                    {t('client_auth.restricted_keys', { defaultValue: 'Restricted keys' })}
                  </span>
                  <strong className={styles.statValue}>{stats.restricted}</strong>
                </div>
                <div className={styles.statItem}>
                  <span className={styles.statLabel}>
                    {t('client_auth.unrestricted_keys', { defaultValue: 'Unrestricted keys' })}
                  </span>
                  <strong className={styles.statValue}>{stats.unrestricted}</strong>
                </div>
              </div>
            </Card>
          </aside>

          <section className={styles.browserPanel}>
            <div className={styles.toolbar}>
              <div className={styles.searchField}>
                <Input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={t('client_auth.search_placeholder', {
                    defaultValue: 'Search by name, key, or auth_index…',
                  })}
                  disabled={loading}
                />
              </div>
              <div className={styles.toolbarSelect}>
                <span className={styles.toolbarLabel}>
                  {t('client_auth.filter_label', { defaultValue: 'Scope' })}
                </span>
                <Select
                  value={scopeFilter}
                  options={scopeFilterOptions}
                  onChange={(value) => setScopeFilter(value as ClientAuthScopeFilter)}
                  ariaLabel={t('client_auth.filter_label', { defaultValue: 'Scope' })}
                />
              </div>
              <div className={styles.toolbarSelect}>
                <span className={styles.toolbarLabel}>
                  {t('client_auth.sort_label', { defaultValue: 'Sort' })}
                </span>
                <Select
                  value={sortMode}
                  options={sortOptions}
                  onChange={(value) => setSortMode(value as ClientAuthSort)}
                  ariaLabel={t('client_auth.sort_label', { defaultValue: 'Sort' })}
                />
              </div>
            </div>

            {visibleKeys.length === 0 && visualValues.apiKeys.length === 0 ? (
              <Card className={styles.emptyCard}>
                <h2 className={styles.emptyTitle}>
                  {t('client_auth.empty_title', { defaultValue: 'No client keys yet' })}
                </h2>
                <p className={styles.emptyDescription}>
                  {t('client_auth.empty_description', {
                    defaultValue:
                      'Create a client key to control how external clients authenticate against this proxy.',
                  })}
                </p>
                <Button
                  onClick={openAddEditor}
                  disabled={disableControls || loading || saving}
                  variant="secondary"
                >
                  {t('client_auth.add_key', { defaultValue: 'Add client key' })}
                </Button>
              </Card>
            ) : visibleKeys.length === 0 ? (
              <Card className={styles.emptyCard}>
                <h2 className={styles.emptyTitle}>
                  {t('client_auth.empty_filtered_title', {
                    defaultValue: 'No keys match the current filters',
                  })}
                </h2>
                <p className={styles.emptyDescription}>
                  {t('client_auth.empty_filtered_description', {
                    defaultValue:
                      'Try clearing the search text or switching the scope filter to reveal existing client keys.',
                  })}
                </p>
                <Button
                  variant="secondary"
                  onClick={() => {
                    setSearch('');
                    setScopeFilter('all');
                  }}
                >
                  {t('client_auth.clear_filters', { defaultValue: 'Clear filters' })}
                </Button>
              </Card>
            ) : (
              <div className={styles.keyGrid}>
                {visibleKeys.map((entry) => (
                  <Card
                    key={entry.id}
                    className={styles.keyCard}
                    title={
                      <div className={styles.keyHeader}>
                        <span className={styles.keyTitle}>
                          {entry.name || t('config_management.visual.api_keys.input_label')}
                        </span>
                        <span
                          className={`${styles.scopePill} ${
                            isRestrictedClientKey(entry) ? styles.scopeRestricted : ''
                          }`}
                        >
                          {isRestrictedClientKey(entry)
                            ? t('client_auth.filter_restricted', { defaultValue: 'Restricted' })
                            : t('client_auth.filter_unrestricted', {
                                defaultValue: 'Unrestricted',
                              })}
                        </span>
                      </div>
                    }
                    extra={
                      <div className={styles.cardActions}>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => handleCopyEntry(entry.key)}
                        >
                          {t('common.copy')}
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => openEditEditor(entry.id)}
                        >
                          {t('config_management.visual.common.edit')}
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => handleDeleteEntry(entry.id)}
                        >
                          {t('config_management.visual.common.delete')}
                        </Button>
                      </div>
                    }
                  >
                    <div className={styles.keyMeta}>
                      <div className={styles.metaLabel}>
                        {t('config_management.visual.api_keys.input_label')}
                      </div>
                      <div className={styles.metaValue}>{maskApiKey(entry.key)}</div>
                    </div>
                    <div className={styles.keyMeta}>
                      <div className={styles.metaLabel}>
                        {t('client_auth.filter_label', { defaultValue: 'Scope' })}
                      </div>
                      <div className={styles.metaValue}>
                        {buildScopeSummary(entry.allowedAuthIndices, credentialOptionMap, t)}
                      </div>
                    </div>
                    {isRestrictedClientKey(entry) ? (
                      <div className={styles.keyMeta}>
                        <div className={styles.metaLabel}>
                          {t('client_auth.allowed_count', { defaultValue: 'Allowed credentials' })}
                        </div>
                        <div className={styles.metaValue}>{entry.allowedAuthIndices.length}</div>
                      </div>
                    ) : null}
                  </Card>
                ))}
              </div>
            )}
          </section>
        </div>
      </div>

      <ApiKeyEditorModal
        open={editorOpen}
        mode={editingKeyId ? 'edit' : 'add'}
        initialEntry={editingEntry}
        sessionKey={editorSessionKey}
        onClose={closeEditor}
        onSave={handleSaveEntry}
        disabled={disableControls || loading || saving}
      />

      <DiffModal
        open={diffModalOpen}
        original={serverYaml}
        modified={mergedYaml}
        onConfirm={handleConfirmSave}
        onCancel={() => setDiffModalOpen(false)}
        loading={saving}
      />
    </div>
  );
}
