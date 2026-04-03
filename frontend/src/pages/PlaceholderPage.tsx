import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';

export function PlaceholderPage({
  titleKey,
  descriptionKey,
}: {
  titleKey: string;
  descriptionKey?: string;
}) {
  const { t } = useTranslation();

  return (
    <Card title={t(titleKey)}>
      <p style={{ color: 'var(--text-secondary)' }}>
        {descriptionKey ? t(descriptionKey) : t('common.loading')}
      </p>
    </Card>
  );
}
