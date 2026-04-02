import { useCallback, useMemo } from 'react';
import { buildRecentMinuteSeries } from '@/utils/usage';
import type { UsagePayload } from './useUsageData';

export interface SparklineData {
  labels: string[];
  datasets: [
    {
      data: number[];
      borderColor: string;
      backgroundColor: string;
      fill: boolean;
      tension: number;
      pointRadius: number;
      borderWidth: number;
    }
  ];
}

export interface SparklineBundle {
  data: SparklineData;
}

export interface UseSparklinesOptions {
  usage: UsagePayload | null;
  loading: boolean;
  nowMs: number;
}

export interface UseSparklinesReturn {
  requestsSparkline: SparklineBundle | null;
  tokensSparkline: SparklineBundle | null;
  rpmSparkline: SparklineBundle | null;
  tpmSparkline: SparklineBundle | null;
  costSparkline: SparklineBundle | null;
}

export function useSparklines({ usage, loading, nowMs }: UseSparklinesOptions): UseSparklinesReturn {
  const lastHourSeries = useMemo(() => {
    if (!usage) return { labels: [], requests: [], tokens: [] };
    return buildRecentMinuteSeries(usage, nowMs, 60);
  }, [nowMs, usage]);

  const buildSparkline = useCallback(
    (
      series: { labels: string[]; data: number[] },
      color: string,
      backgroundColor: string
    ): SparklineBundle | null => {
      if (loading || !series?.data?.length) {
        return null;
      }
      const sliceStart = Math.max(series.data.length - 60, 0);
      const labels = series.labels.slice(sliceStart);
      const points = series.data.slice(sliceStart);
      return {
        data: {
          labels,
          datasets: [
            {
              data: points,
              borderColor: color,
              backgroundColor,
              fill: true,
              tension: 0.45,
              pointRadius: 0,
              borderWidth: 2
            }
          ]
        }
      };
    },
    [loading]
  );

  const requestsSparkline = useMemo(
    () =>
      buildSparkline(
        { labels: lastHourSeries.labels, data: lastHourSeries.requests },
        '#8b8680',
        'rgba(139, 134, 128, 0.18)'
      ),
    [buildSparkline, lastHourSeries.labels, lastHourSeries.requests]
  );

  const tokensSparkline = useMemo(
    () =>
      buildSparkline(
        { labels: lastHourSeries.labels, data: lastHourSeries.tokens },
        '#8b5cf6',
        'rgba(139, 92, 246, 0.18)'
      ),
    [buildSparkline, lastHourSeries.labels, lastHourSeries.tokens]
  );

  const rpmSparkline = useMemo(
    () =>
      buildSparkline(
        { labels: lastHourSeries.labels, data: lastHourSeries.requests },
        '#22c55e',
        'rgba(34, 197, 94, 0.18)'
      ),
    [buildSparkline, lastHourSeries.labels, lastHourSeries.requests]
  );

  const tpmSparkline = useMemo(
    () =>
      buildSparkline(
        { labels: lastHourSeries.labels, data: lastHourSeries.tokens },
        '#f97316',
        'rgba(249, 115, 22, 0.18)'
      ),
    [buildSparkline, lastHourSeries.labels, lastHourSeries.tokens]
  );

  const costSparkline = useMemo(
    () =>
      buildSparkline(
        { labels: lastHourSeries.labels, data: lastHourSeries.tokens },
        '#f59e0b',
        'rgba(245, 158, 11, 0.18)'
      ),
    [buildSparkline, lastHourSeries.labels, lastHourSeries.tokens]
  );

  return {
    requestsSparkline,
    tokensSparkline,
    rpmSparkline,
    tpmSparkline,
    costSparkline
  };
}
