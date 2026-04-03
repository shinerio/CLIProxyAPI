# Client Authentication Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated `Client Authentication` page below `Config Panel`, move `auth-dir` and `api-keys` management there, and remove the old auth section from the config panel.

**Architecture:** Keep `config.yaml` as the only source of truth. The new page reads the existing YAML through `configFileApi`, uses `useVisualConfig` to parse and apply auth-related values, and saves through the existing diff-confirm-save flow. UI-specific browsing logic for client keys lives in a focused client-auth feature area, while `ConfigPage` simply stops rendering the old auth section.

**Tech Stack:** React 19, TypeScript, React Router, Zustand stores, SCSS modules, existing config YAML APIs and `useVisualConfig`.

---

## File Map

### Create

- `frontend/src/pages/ClientAuthPage.tsx`
- `frontend/src/pages/ClientAuthPage.module.scss`
- `frontend/src/features/clientAuth/utils.ts`

### Modify

- `frontend/src/router/MainRoutes.tsx`
- `frontend/src/components/layout/MainLayout.tsx`
- `frontend/src/components/ui/icons.tsx`
- `frontend/src/components/config/VisualConfigEditor.tsx`
- `frontend/src/components/config/VisualConfigEditorBlocks.tsx`
- `frontend/src/i18n/locales/en.json`
- `frontend/src/i18n/locales/zh-CN.json`
- `frontend/src/i18n/locales/ru.json`

### Verify

- `frontend/package.json`

## Constraints and Verification Strategy

- The frontend currently has no automated unit/integration test harness.
- Follow repository guidance: verify with `npm run lint`, `npm run type-check`, `npm run build`, then manual browser checks in `npm run dev`.
- Do not add backend changes or a new config endpoint.
- Do not expose `remote-management.secret-key` on the new page.

### Task 1: Add Navigation and Route Entry

**Files:**
- Modify: `frontend/src/router/MainRoutes.tsx`
- Modify: `frontend/src/components/layout/MainLayout.tsx`
- Modify: `frontend/src/components/ui/icons.tsx`
- Modify: `frontend/src/i18n/locales/en.json`
- Modify: `frontend/src/i18n/locales/zh-CN.json`
- Modify: `frontend/src/i18n/locales/ru.json`

- [ ] **Step 1: Add a dedicated sidebar icon**

```tsx
export function IconSidebarClientAuth({ size = 20, ...props }: IconProps) {
  return (
    <svg {...sidebarSvgProps} width={size} height={size} {...props}>
      <circle cx="8" cy="11" r="3.5" />
      <path d="M11.5 11H21" />
      <path d="M16 7.5v7" />
      <path d="M19 9.5v3" />
    </svg>
  );
}
```

- [ ] **Step 2: Wire the new route**

```tsx
import { ClientAuthPage } from '@/pages/ClientAuthPage';

const mainRoutes = [
  { path: '/config', element: <ConfigPage /> },
  { path: '/client-auth', element: <ClientAuthPage /> },
  { path: '/ai-providers', element: <AiProvidersPage /> },
];
```

- [ ] **Step 3: Insert the new sidebar item below config panel**

```tsx
const sidebarIcons: Record<string, ReactNode> = {
  config: <IconSidebarConfig size={18} />,
  clientAuth: <IconSidebarClientAuth size={18} />,
};

const navItems = [
  { path: '/', label: t('nav.dashboard'), icon: sidebarIcons.dashboard },
  { path: '/config', label: t('nav.config_management'), icon: sidebarIcons.config },
  { path: '/client-auth', label: t('nav.client_auth'), icon: sidebarIcons.clientAuth },
  { path: '/ai-providers', label: t('nav.ai_providers'), icon: sidebarIcons.aiProviders },
];
```

- [ ] **Step 4: Update route ordering logic so the new page animates and highlights correctly**

```tsx
const exactIndex = navOrder.indexOf(normalizedPath);
if (exactIndex !== -1) return exactIndex;
const nestedIndex = navOrder.findIndex(
  (path) => path !== '/' && normalizedPath.startsWith(`${path}/`)
);
```

No special nested transition handling is needed beyond normal vertical transitions.

- [ ] **Step 5: Add locale strings for the nav label and new page copy**

```json
{
  "nav": {
    "client_auth": "Client Authentication"
  },
  "client_auth": {
    "title": "Client Authentication",
    "description": "Manage auth-dir and client API keys without mixing in remote management settings."
  }
}
```

- [ ] **Step 6: Verify the navigation changes compile cleanly**

Run: `cd frontend && npm run type-check`

Expected: TypeScript completes without new route/import errors.

### Task 2: Refactor the Existing API Key Editor for Reuse

**Files:**
- Modify: `frontend/src/components/config/VisualConfigEditorBlocks.tsx`
- Create: `frontend/src/features/clientAuth/utils.ts`

- [ ] **Step 1: Extract reusable client-auth helpers into a focused feature utility file**

```ts
import type { VisualApiKeyEntry } from '@/types/visualConfig';

export type ClientAuthScopeFilter = 'all' | 'restricted' | 'unrestricted';
export type ClientAuthSort = 'default' | 'name';

export function isRestrictedClientKey(entry: VisualApiKeyEntry) {
  return entry.allowedAuthIndices.length > 0;
}

export function matchesClientAuthSearch(entry: VisualApiKeyEntry, query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return (
    entry.name.toLowerCase().includes(needle) ||
    entry.key.toLowerCase().includes(needle) ||
    entry.allowedAuthIndices.some((item) => item.toLowerCase().includes(needle))
  );
}
```

- [ ] **Step 2: Add list-mode props to the existing API key editor so the new page can own browsing state**

```tsx
interface ApiKeyEditorModalProps {
  open: boolean;
  mode: 'add' | 'edit';
  initialEntry?: VisualApiKeyEntry | null;
  disabled?: boolean;
  apiKeys: VisualApiKeyEntry[];
  onClose: () => void;
  onSave: (nextEntry: VisualApiKeyEntry) => void;
}
```

Keep the existing modal behavior intact by letting `ApiKeysCardEditor` render this extracted modal internally, while `ClientAuthPage` imports the same modal for page-owned add/edit actions.

- [ ] **Step 3: Add credential search inside the modal scope chooser**

```tsx
const [credentialSearch, setCredentialSearch] = useState('');

const visibleCredentialOptions = useMemo(
  () =>
    credentialOptions.filter((option) => {
      const needle = credentialSearch.trim().toLowerCase();
      if (!needle) return true;
      return (
        option.title.toLowerCase().includes(needle) ||
        option.subtitle.toLowerCase().includes(needle) ||
        option.authIndex.toLowerCase().includes(needle)
      );
    }),
  [credentialOptions, credentialSearch]
);
```

- [ ] **Step 4: Keep scope semantics unchanged during refactor**

```tsx
const normalizedSelection = Array.from(
  new Set(selectedAuthIndices.map((item) => item.trim()).filter(Boolean))
);

const nextEntry: VisualApiKeyEntry = {
  id: editingApiKeyId ?? makeClientId(),
  name: nameInputValue.trim(),
  key: trimmed,
  allowedAuthIndices: normalizedSelection,
};
```

- [ ] **Step 5: Verify the refactor has not broken the current editor contract**

Run: `cd frontend && npm run lint`

Expected: ESLint passes and `VisualConfigEditorBlocks.tsx` keeps a valid public interface.

### Task 3: Build the Dedicated Client Authentication Page

**Files:**
- Create: `frontend/src/pages/ClientAuthPage.tsx`
- Create: `frontend/src/pages/ClientAuthPage.module.scss`
- Modify: `frontend/src/components/config/VisualConfigEditorBlocks.tsx`
- Modify: `frontend/src/i18n/locales/en.json`
- Modify: `frontend/src/i18n/locales/zh-CN.json`
- Modify: `frontend/src/i18n/locales/ru.json`

- [ ] **Step 1: Create page-local state for YAML loading, auth editing, and key browsing**

```tsx
const {
  visualValues,
  visualDirty,
  visualParseError,
  loadVisualValuesFromYaml,
  applyVisualChangesToYaml,
  setVisualValues,
} = useVisualConfig();

const [content, setContent] = useState('');
const [loading, setLoading] = useState(true);
const [saving, setSaving] = useState(false);
const [error, setError] = useState('');
const [serverYaml, setServerYaml] = useState('');
const [mergedYaml, setMergedYaml] = useState('');
const [search, setSearch] = useState('');
const [scopeFilter, setScopeFilter] = useState<ClientAuthScopeFilter>('all');
const [sortMode, setSortMode] = useState<ClientAuthSort>('default');
const [diffModalOpen, setDiffModalOpen] = useState(false);
const [editorOpen, setEditorOpen] = useState(false);
const [editingKeyId, setEditingKeyId] = useState<string | null>(null);
```

- [ ] **Step 2: Reuse the config file fetch and save lifecycle**

```tsx
const loadConfig = useCallback(async () => {
  setLoading(true);
  setError('');
  try {
    const data = await configFileApi.fetchConfigYaml();
    setContent(data);
    setServerYaml(data);
    setMergedYaml(data);
    loadVisualValuesFromYaml(data);
  } catch (err) {
    setError(err instanceof Error ? err.message : t('notification.refresh_failed'));
  } finally {
    setLoading(false);
  }
}, [loadVisualValuesFromYaml, t]);
```

```tsx
const handleSave = async () => {
  const latestServerYaml = await configFileApi.fetchConfigYaml();
  const nextMergedYaml = applyVisualChangesToYaml(latestServerYaml);
  setServerYaml(latestServerYaml);
  setMergedYaml(nextMergedYaml);
  setDiffModalOpen(true);
};
```

```tsx
const handleConfirmSave = async () => {
  await configFileApi.saveConfigYaml(mergedYaml);
  useConfigStore.getState().clearCache();
  await useConfigStore.getState().fetchConfig(undefined, true);
};
```

- [ ] **Step 3: Build filtered and sorted key browsing on the page, not inside the modal**

```tsx
const visibleKeys = useMemo(() => {
  const filtered = visualValues.apiKeys.filter((entry) => {
    if (!matchesClientAuthSearch(entry, search)) return false;
    if (scopeFilter === 'restricted') return isRestrictedClientKey(entry);
    if (scopeFilter === 'unrestricted') return !isRestrictedClientKey(entry);
    return true;
  });

  if (sortMode === 'name') {
    return [...filtered].sort((left, right) =>
      (left.name || left.key).localeCompare(right.name || right.key)
    );
  }

  return filtered;
}, [scopeFilter, search, sortMode, visualValues.apiKeys]);
```

- [ ] **Step 4: Render the new two-column layout**

```tsx
<div className={styles.workspace}>
  <aside className={styles.summaryRail}>
    <Card>
      <Input
        label={t('client_auth.auth_dir')}
        value={visualValues.authDir}
        onChange={(e) => setVisualValues({ authDir: e.target.value })}
      />
    </Card>
    <Card>{/* counts and page help */}</Card>
  </aside>

  <section className={styles.browserPanel}>
    <div className={styles.toolbar}>{/* search, scope filter, sort */}</div>
    <div className={styles.keyGrid}>{/* key cards */}</div>
  </section>
</div>
```

- [ ] **Step 5: Connect add/edit actions to the extracted API key modal**

```tsx
<ApiKeyEditorModal
  open={editorOpen}
  mode={editingKeyId ? 'edit' : 'add'}
  initialEntry={
    editingKeyId ? visualValues.apiKeys.find((entry) => entry.id === editingKeyId) ?? null : null
  }
  apiKeys={visualValues.apiKeys}
  onClose={() => {
    setEditorOpen(false);
    setEditingKeyId(null);
  }}
  onSave={(nextEntry) => {
    const nextKeys = editingKeyId
      ? visualValues.apiKeys.map((entry) => (entry.id === editingKeyId ? nextEntry : entry))
      : [...visualValues.apiKeys, nextEntry];
    setVisualValues({ apiKeys: nextKeys });
  }}
  disabled={disableControls}
/>
```

- [ ] **Step 6: Add page styling for dense browsing**

```scss
.workspace {
  display: grid;
  grid-template-columns: minmax(280px, 320px) minmax(0, 1fr);
  gap: 24px;
}

.keyGrid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}
```

- [ ] **Step 7: Verify the page builds**

Run: `cd frontend && npm run build`

Expected: Vite build completes and the new page has no SCSS or route integration errors.

### Task 4: Remove the Old Auth Section from the Config Panel

**Files:**
- Modify: `frontend/src/components/config/VisualConfigEditor.tsx`
- Modify: `frontend/src/i18n/locales/en.json`
- Modify: `frontend/src/i18n/locales/zh-CN.json`
- Modify: `frontend/src/i18n/locales/ru.json`

- [ ] **Step 1: Remove the auth section id from the visual editor section model**

```tsx
type VisualSectionId =
  | 'server'
  | 'tls'
  | 'remote'
  | 'system'
  | 'network'
  | 'quota'
  | 'streaming'
  | 'payload';
```

- [ ] **Step 2: Remove the auth quick-jump item and rendered section block**

```tsx
{
  id: 'remote',
  title: t('config_management.visual.sections.remote.title'),
  description: t('config_management.visual.sections.remote.description'),
  icon: IconSatellite,
  errorCount: 0,
},
{
  id: 'system',
  title: t('config_management.visual.sections.system.title'),
  description: t('config_management.visual.sections.system.description'),
  icon: IconDiamond,
  errorCount: countErrors(['logsMaxTotalSizeMb']),
},
```

Delete the full `ConfigSection` whose `id="auth"`.

- [ ] **Step 3: Keep auth parsing in `useVisualConfig` unchanged**

No code removal in `useVisualConfig` is needed. The new page depends on the same `authDir` and `apiKeys` parsing/apply logic.

- [ ] **Step 4: Remove or stop using now-stale config-panel copy if it becomes unreachable**

Keep locale cleanup minimal: remove only strings that become obviously dead if they are not reused by the new page.

- [ ] **Step 5: Verify the config panel still renders in visual mode**

Run: `cd frontend && npm run type-check`

Expected: `VisualConfigEditor` compiles without dangling `auth` references.

### Task 5: Final Verification and Manual Regression

**Files:**
- Verify only

- [ ] **Step 1: Run the frontend verification gate**

Run: `cd frontend && npm run lint && npm run type-check && npm run build`

Expected: all three commands pass.

- [ ] **Step 2: Start the app for manual verification**

Run: `cd frontend && npm run dev`

Expected: Vite serves the app locally and the new route is reachable.

- [ ] **Step 3: Manually verify sidebar behavior**

Check:

- `Client Authentication` appears directly below `Config Panel`
- active state works on `/client-auth`
- switching between `/config`, `/client-auth`, and `/auth-files` feels correct

- [ ] **Step 4: Manually verify client auth behavior**

Check:

- existing `auth-dir` loads
- existing client API keys load
- add key works
- edit key works
- delete key works
- copy key works
- credential search inside the modal works
- scope filter and sort work

- [ ] **Step 5: Manually verify save boundaries**

Check:

- saving `/client-auth` updates only `auth-dir` and `api-keys`
- `remote-management.secret-key` does not appear on the page
- `/config` no longer renders the old auth section
- diff confirmation still appears before save

- [ ] **Step 6: Commit the implementation in focused slices**

Recommended commit sequence:

```bash
git add frontend/src/components/ui/icons.tsx frontend/src/router/MainRoutes.tsx frontend/src/components/layout/MainLayout.tsx frontend/src/i18n/locales/en.json frontend/src/i18n/locales/zh-CN.json frontend/src/i18n/locales/ru.json
git commit -m "feat(frontend): add client authentication navigation"
```

```bash
git add frontend/src/features/clientAuth/utils.ts frontend/src/components/config/VisualConfigEditorBlocks.tsx frontend/src/pages/ClientAuthPage.tsx frontend/src/pages/ClientAuthPage.module.scss
git commit -m "feat(frontend): add client authentication page"
```

```bash
git add frontend/src/components/config/VisualConfigEditor.tsx
git commit -m "refactor(frontend): remove auth section from config panel"
```

## Self-Review Notes

- Spec coverage:
  - sidebar addition: covered in Task 1
  - dedicated page: covered in Task 3
  - auth-only ownership boundary: covered in Tasks 3 and 4
  - config panel cleanup: covered in Task 4
  - verification and regression checks: covered in Task 5
- Placeholder scan: no `TODO`, `TBD`, or deferred implementation notes remain
- Type consistency:
  - route path uses `/client-auth` consistently
  - page copy uses `client_auth` locale namespace consistently
  - auth data continues to use `authDir` and `apiKeys` from `useVisualConfig`
