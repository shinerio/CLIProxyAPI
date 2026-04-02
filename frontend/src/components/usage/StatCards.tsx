import { useMemo, type CSSProperties, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Line } from 'react-chartjs-2';
import {
  IconDiamond,
  IconDollarSign,
  IconSatellite,
  IconTimer,
  IconTrendingUp,
} from '@/components/ui/icons';
import {
  calculateAllTimePerMinuteRates,
  calculatePeakPerMinuteStats,
  calculateUsageSummaryForTimeRange,
  formatCompactNumber,
  formatPerMinuteValue,
  formatUsd,
  calculateRecentPerMinuteRates,
  calculateTokenBreakdown,
  calculateTotalCost,
  type UsageTimeRange,
  type ModelPrice,
} from '@/utils/usage';
import { sparklineOptions } from '@/utils/usage/chartConfig';
import type { UsagePayload } from './hooks/useUsageData';
import type { SparklineBundle } from './hooks/useSparklines';
import styles from '@/pages/UsagePage.module.scss';

interface StatCardData {
  key: string;
  label: string;
  icon: ReactNode;
  accent: string;
  accentSoft: string;
  accentBorder: string;
  value: string;
  meta?: ReactNode;
  trend: SparklineBundle | null;
}

export interface StatCardsProps {
  usage: UsagePayload | null;
  loading: boolean;
  modelPrices: Record<string, ModelPrice>;
  timeRange: UsageTimeRange;
  nowMs: number;
  sparklines: {
    requests: SparklineBundle | null;
    tokens: SparklineBundle | null;
    rpm: SparklineBundle | null;
    tpm: SparklineBundle | null;
    cost: SparklineBundle | null;
  };
}

const FIXED_WINDOW_MINUTES: Record<Exclude<UsageTimeRange, 'all'>, number> = {
  '7h': 7 * 60,
  '24h': 24 * 60,
  '7d': 7 * 24 * 60,
};

export function StatCards({
  usage,
  loading,
  modelPrices,
  timeRange,
  nowMs,
  sparklines,
}: StatCardsProps) {
  const { t } = useTranslation();

  const hasPrices = Object.keys(modelPrices).length > 0;
  const rateLabel = `${t('usage_stats.rpm_30m')} (${t(`usage_stats.range_${timeRange}`)})`;
  const tokenRateLabel = `${t('usage_stats.tpm_30m')} (${t(`usage_stats.range_${timeRange}`)})`;
  const peakRequestsLabel = `${t('usage_stats.max_requests_per_minute')} (${t(`usage_stats.range_${timeRange}`)})`;
  const peakTokensLabel = `${t('usage_stats.max_tokens_per_minute')} (${t(`usage_stats.range_${timeRange}`)})`;

  const { tokenBreakdown, rateStats, totalCost, summaryStats, peakStats } = useMemo(() => {
    const empty = {
      tokenBreakdown: { cachedTokens: 0, reasoningTokens: 0 },
      rateStats: { rpm: 0, tpm: 0, windowMinutes: 0, requestCount: 0, tokenCount: 0 },
      totalCost: 0,
      summaryStats: { totalRequests: 0, successCount: 0, failureCount: 0, totalTokens: 0 },
      peakStats: { maxRequestsPerMinute: 0, maxTokensPerMinute: 0 },
    };

    if (!usage) return empty;

    const windowMinutes = timeRange === 'all' ? 0 : FIXED_WINDOW_MINUTES[timeRange];
    return {
      tokenBreakdown: calculateTokenBreakdown(usage),
      summaryStats: calculateUsageSummaryForTimeRange(usage, timeRange, nowMs),
      peakStats: calculatePeakPerMinuteStats(usage, timeRange, nowMs),
      rateStats:
        timeRange === 'all'
          ? calculateAllTimePerMinuteRates(usage)
          : calculateRecentPerMinuteRates(windowMinutes, usage),
      totalCost: hasPrices ? calculateTotalCost(usage, modelPrices) : 0,
    };
  }, [hasPrices, modelPrices, nowMs, timeRange, usage]);

  const statsCards: StatCardData[] = [
    {
      key: 'requests',
      label: t('usage_stats.total_requests'),
      icon: <IconSatellite size={16} />,
      accent: '#8b8680',
      accentSoft: 'rgba(139, 134, 128, 0.18)',
      accentBorder: 'rgba(139, 134, 128, 0.35)',
      value: loading ? '-' : summaryStats.totalRequests.toLocaleString(),
      meta: (
        <>
          <span className={styles.statMetaItem}>
            <span className={styles.statMetaDot} style={{ backgroundColor: '#10b981' }} />
            {t('usage_stats.success_requests')}:{' '}
            {loading ? '-' : summaryStats.successCount.toLocaleString()}
          </span>
          <span className={styles.statMetaItem}>
            <span className={styles.statMetaDot} style={{ backgroundColor: '#c65746' }} />
            {t('usage_stats.failed_requests')}:{' '}
            {loading ? '-' : summaryStats.failureCount.toLocaleString()}
          </span>
        </>
      ),
      trend: sparklines.requests,
    },
    {
      key: 'tokens',
      label: t('usage_stats.total_tokens'),
      icon: <IconDiamond size={16} />,
      accent: '#8b5cf6',
      accentSoft: 'rgba(139, 92, 246, 0.18)',
      accentBorder: 'rgba(139, 92, 246, 0.35)',
      value: loading ? '-' : formatCompactNumber(summaryStats.totalTokens),
      meta: (
        <>
          <span className={styles.statMetaItem}>
            {t('usage_stats.cached_tokens')}:{' '}
            {loading ? '-' : formatCompactNumber(tokenBreakdown.cachedTokens)}
          </span>
          <span className={styles.statMetaItem}>
            {t('usage_stats.reasoning_tokens')}:{' '}
            {loading ? '-' : formatCompactNumber(tokenBreakdown.reasoningTokens)}
          </span>
        </>
      ),
      trend: sparklines.tokens,
    },
    {
      key: 'rpm',
      label: rateLabel,
      icon: <IconTimer size={16} />,
      accent: '#22c55e',
      accentSoft: 'rgba(34, 197, 94, 0.18)',
      accentBorder: 'rgba(34, 197, 94, 0.32)',
      value: loading ? '-' : formatPerMinuteValue(rateStats.rpm),
      meta: (
        <span className={styles.statMetaItem}>
          {t('usage_stats.total_requests')}:{' '}
          {loading ? '-' : rateStats.requestCount.toLocaleString()}
        </span>
      ),
      trend: sparklines.rpm,
    },
    {
      key: 'tpm',
      label: tokenRateLabel,
      icon: <IconTrendingUp size={16} />,
      accent: '#f97316',
      accentSoft: 'rgba(249, 115, 22, 0.18)',
      accentBorder: 'rgba(249, 115, 22, 0.32)',
      value: loading ? '-' : formatPerMinuteValue(rateStats.tpm),
      meta: (
        <span className={styles.statMetaItem}>
          {t('usage_stats.total_tokens')}:{' '}
          {loading ? '-' : formatCompactNumber(rateStats.tokenCount)}
        </span>
      ),
      trend: sparklines.tpm,
    },
    {
      key: 'peak-rpm',
      label: peakRequestsLabel,
      icon: <IconTimer size={16} />,
      accent: '#0f766e',
      accentSoft: 'rgba(15, 118, 110, 0.18)',
      accentBorder: 'rgba(15, 118, 110, 0.32)',
      value: loading ? '-' : formatPerMinuteValue(peakStats.maxRequestsPerMinute),
      meta: <span className={styles.statMetaItem}>{t('usage_stats.range_peak_hint')}</span>,
      trend: sparklines.requests,
    },
    {
      key: 'peak-tpm',
      label: peakTokensLabel,
      icon: <IconTrendingUp size={16} />,
      accent: '#ea580c',
      accentSoft: 'rgba(234, 88, 12, 0.18)',
      accentBorder: 'rgba(234, 88, 12, 0.32)',
      value: loading ? '-' : formatCompactNumber(peakStats.maxTokensPerMinute),
      meta: <span className={styles.statMetaItem}>{t('usage_stats.range_peak_hint')}</span>,
      trend: sparklines.tokens,
    },
    {
      key: 'cost',
      label: t('usage_stats.total_cost'),
      icon: <IconDollarSign size={16} />,
      accent: '#f59e0b',
      accentSoft: 'rgba(245, 158, 11, 0.18)',
      accentBorder: 'rgba(245, 158, 11, 0.32)',
      value: loading ? '-' : hasPrices ? formatUsd(totalCost) : '--',
      meta: (
        <>
          <span className={styles.statMetaItem}>
            {t('usage_stats.total_tokens')}:{' '}
            {loading ? '-' : formatCompactNumber(summaryStats.totalTokens)}
          </span>
          {!hasPrices && (
            <span className={`${styles.statMetaItem} ${styles.statSubtle}`}>
              {t('usage_stats.cost_need_price')}
            </span>
          )}
        </>
      ),
      trend: hasPrices ? sparklines.cost : null,
    },
  ];

  return (
    <div className={styles.statsGrid}>
      {statsCards.map((card) => (
        <div
          key={card.key}
          className={styles.statCard}
          style={
            {
              '--accent': card.accent,
              '--accent-soft': card.accentSoft,
              '--accent-border': card.accentBorder,
            } as CSSProperties
          }
        >
          <div className={styles.statCardHeader}>
            <div className={styles.statLabelGroup}>
              <span className={styles.statLabel}>{card.label}</span>
            </div>
            <span className={styles.statIconBadge}>{card.icon}</span>
          </div>
          <div className={styles.statValue}>{card.value}</div>
          {card.meta && <div className={styles.statMetaRow}>{card.meta}</div>}
          <div className={styles.statTrend}>
            {card.trend ? (
              <Line
                className={styles.sparkline}
                data={card.trend.data}
                options={sparklineOptions}
              />
            ) : (
              <div className={styles.statTrendPlaceholder}></div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
