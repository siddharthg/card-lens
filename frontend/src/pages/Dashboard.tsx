import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, Tooltip,
  ResponsiveContainer, CartesianGrid,
} from 'recharts';
import { format, subMonths } from 'date-fns';
import { api } from '../api/client';
import { formatINR, BANK_COLORS } from '../types';
import type { SpendSummary, MonthTrend, CreditCard } from '../types';

const CATEGORY_COLORS = [
  '#3B82F6', '#EF4444', '#10B981', '#F59E0B', '#8B5CF6',
  '#EC4899', '#06B6D4', '#F97316', '#84CC16', '#6366F1',
  '#14B8A6', '#E11D48', '#A855F7', '#0EA5E9', '#D97706',
];

export default function Dashboard() {
  const [selectedMonth, setSelectedMonth] = useState('');
  const [periodMode, setPeriodMode] = useState<'month' | 'cycle'>('month');
  const [selectedCard, setSelectedCard] = useState('');

  const { data: cards } = useQuery<CreditCard[]>({
    queryKey: ['cards'],
    queryFn: () => api.cards.list(),
  });

  const { data: trends } = useQuery<MonthTrend[]>({
    queryKey: ['trends'],
    queryFn: () => api.analytics.trends({ months: 12 }),
  });

  // Default to the latest month with transactions
  useEffect(() => {
    if (selectedMonth || !trends) return;
    const latest = trends
      .filter((t) => t.total > 0)
      .sort((a, b) => b.month.localeCompare(a.month))[0];
    setSelectedMonth(latest?.month ?? format(new Date(), 'yyyy-MM'));
  }, [trends, selectedMonth]);

  const effectiveMonth = selectedMonth || format(new Date(), 'yyyy-MM');

  const { data: summary, isLoading } = useQuery<SpendSummary>({
    queryKey: ['summary', effectiveMonth, periodMode, selectedCard],
    queryFn: () =>
      api.analytics.summary({
        date: effectiveMonth,
        card_id: selectedCard || undefined,
        ...(periodMode === 'cycle' && selectedCard ? { period: 'cycle' } : {}),
      }),
    enabled: !!selectedMonth,
  });

  // Month selector options (last 12 months)
  const months = Array.from({ length: 12 }, (_, i) => {
    const d = subMonths(new Date(), i);
    return { value: format(d, 'yyyy-MM'), label: format(d, 'MMM yyyy') };
  });

  // Previous month for comparison
  const prevMonth = format(subMonths(new Date(effectiveMonth + '-01'), 1), 'yyyy-MM');
  const { data: prevSummary } = useQuery<SpendSummary>({
    queryKey: ['summary', prevMonth, 'month', ''],
    queryFn: () => api.analytics.summary({ date: prevMonth }),
  });

  const percentChange = prevSummary?.total_spend
    ? (((summary?.total_spend ?? 0) - prevSummary.total_spend) / prevSummary.total_spend) * 100
    : 0;

  // Category pie data
  const categoryData = Object.entries(summary?.by_category ?? {})
    .map(([name, value]) => ({ name, value }))
    .sort((a, b) => b.value - a.value);

  // Daily bar data
  const dailyData = Object.entries(summary?.daily_spend ?? {})
    .map(([date, amount]) => ({ date: date.slice(8), amount }))
    .sort((a, b) => a.date.localeCompare(b.date));

  // Spender data
  const spenderData = Object.entries(summary?.by_spender ?? {})
    .map(([name, value]) => ({ name, value }))
    .sort((a, b) => b.value - a.value);

  // Card-wise data
  const cardMap = new Map(cards?.map((c) => [c.id, c]) ?? []);
  const cardData = Object.entries(summary?.by_card ?? {})
    .map(([id, value]) => {
      const card = cardMap.get(id);
      return {
        name: card ? `${card.bank} ${card.card_name}` : id.slice(0, 8),
        value,
        color: card ? (BANK_COLORS[card.bank] ?? '#6B7280') : '#6B7280',
      };
    })
    .sort((a, b) => b.value - a.value);

  // Period display
  const periodLabel = summary?.period ?? selectedMonth;

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-8">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <div className="flex items-center gap-3">
          {/* Period toggle */}
          <div className="flex bg-gray-800 rounded-lg p-0.5">
            <button
              className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                periodMode === 'month' ? 'bg-blue-600 text-white' : 'text-gray-400'
              }`}
              onClick={() => setPeriodMode('month')}
            >
              Month
            </button>
            <button
              className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                periodMode === 'cycle' ? 'bg-blue-600 text-white' : 'text-gray-400'
              }`}
              onClick={() => setPeriodMode('cycle')}
              disabled={!selectedCard}
              title={!selectedCard ? 'Select a card to use cycle view' : ''}
            >
              Cycle
            </button>
          </div>

          {/* Card filter */}
          <select
            className="select text-sm"
            value={selectedCard}
            onChange={(e) => setSelectedCard(e.target.value)}
          >
            <option value="">All Cards</option>
            {cards?.map((c) => (
              <option key={c.id} value={c.id}>{c.bank} {c.card_name}</option>
            ))}
          </select>

          {/* Month selector */}
          <select
            value={selectedMonth}
            onChange={(e) => setSelectedMonth(e.target.value)}
            className="select text-sm"
          >
            {months.map((m) => (
              <option key={m.value} value={m.value}>{m.label}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Period indicator for cycle mode */}
      {periodMode === 'cycle' && summary?.period && (
        <div className="text-sm text-gray-400 mb-4">
          Cycle: {summary.period}
        </div>
      )}

      {isLoading ? (
        <div className="text-gray-500">Loading...</div>
      ) : (
        <div className="space-y-6">
          {/* Stats row */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {/* Total Spend */}
            <div className="card">
              <p className="text-sm text-gray-400 mb-1">Total Spend</p>
              <p className="text-3xl font-bold">{formatINR(summary?.total_spend ?? 0)}</p>
              {prevSummary && percentChange !== 0 && (
                <p className={`text-sm mt-2 ${percentChange > 0 ? 'text-red-400' : 'text-green-400'}`}>
                  {percentChange > 0 ? '↑' : '↓'} {Math.abs(percentChange).toFixed(1)}% vs last month
                </p>
              )}
            </div>

            {/* Transaction Count */}
            <div className="card">
              <p className="text-sm text-gray-400 mb-1">Transactions</p>
              <p className="text-3xl font-bold">
                {Object.values(summary?.daily_spend ?? {}).length > 0
                  ? (summary?.top_merchants?.reduce((s, m) => s + m.count, 0) ?? 0)
                  : 0}
              </p>
              <p className="text-sm text-gray-500 mt-2">
                {Object.keys(summary?.daily_spend ?? {}).length} active days
              </p>
            </div>

            {/* Top Category */}
            <div className="card">
              <p className="text-sm text-gray-400 mb-1">Top Category</p>
              {categoryData.length > 0 ? (
                <>
                  <p className="text-3xl font-bold">{formatINR(categoryData[0].value)}</p>
                  <p className="text-sm text-gray-500 mt-2">{categoryData[0].name}</p>
                </>
              ) : (
                <p className="text-3xl font-bold text-gray-600">-</p>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Category Breakdown */}
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">By Category</h2>
              {categoryData.length === 0 ? (
                <p className="text-gray-500 text-sm">No data for this period</p>
              ) : (
                <>
                  <ResponsiveContainer width="100%" height={280}>
                    <PieChart>
                      <Pie
                        data={categoryData}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={100}
                        paddingAngle={2}
                        dataKey="value"
                      >
                        {categoryData.map((_, index) => (
                          <Cell key={index} fill={CATEGORY_COLORS[index % CATEGORY_COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip
                        formatter={(value: number) => formatINR(value)}
                        contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="grid grid-cols-2 gap-2 mt-2">
                    {categoryData.slice(0, 6).map((c, i) => (
                      <div key={c.name} className="flex items-center gap-2 text-sm">
                        <div
                          className="w-2.5 h-2.5 rounded-full flex-shrink-0"
                          style={{ backgroundColor: CATEGORY_COLORS[i % CATEGORY_COLORS.length] }}
                        />
                        <span className="text-gray-400 truncate">{c.name}</span>
                        <span className="ml-auto font-medium">{formatINR(c.value)}</span>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>

            {/* Daily Spend */}
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Daily Spend</h2>
              {dailyData.length === 0 ? (
                <p className="text-gray-500 text-sm">No data for this period</p>
              ) : (
                <ResponsiveContainer width="100%" height={340}>
                  <BarChart data={dailyData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
                    <XAxis dataKey="date" tick={{ fill: '#9CA3AF', fontSize: 11 }} />
                    <YAxis tick={{ fill: '#9CA3AF', fontSize: 11 }} />
                    <Tooltip
                      formatter={(value: number) => formatINR(value)}
                      contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                    />
                    <Bar dataKey="amount" fill="#3B82F6" radius={[3, 3, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Top Merchants */}
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Top Merchants</h2>
              {(summary?.top_merchants?.length ?? 0) === 0 ? (
                <p className="text-gray-500 text-sm">No data for this period</p>
              ) : (
                <div className="space-y-3">
                  {summary!.top_merchants.slice(0, 5).map((m, i) => (
                    <div key={i} className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <span className="text-lg font-bold text-gray-600 w-6">{i + 1}</span>
                        <div>
                          <p className="font-medium">{m.merchant}</p>
                          <p className="text-xs text-gray-500">{m.category} &middot; {m.count} txns</p>
                        </div>
                      </div>
                      <p className="font-semibold">{formatINR(m.amount)}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Card-wise + Spender Split */}
            <div className="space-y-6">
              {/* Spender Split */}
              {spenderData.length > 1 && (
                <div className="card">
                  <h2 className="text-lg font-semibold mb-4">By Spender</h2>
                  <div className="space-y-2">
                    {spenderData.map((sp) => {
                      const pct = (sp.value / (summary?.total_spend ?? 1)) * 100;
                      return (
                        <div key={sp.name}>
                          <div className="flex justify-between text-sm mb-1">
                            <span>{sp.name}</span>
                            <span className="font-medium">{formatINR(sp.value)}</span>
                          </div>
                          <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                            <div
                              className="h-full bg-blue-500 rounded-full"
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* Card-wise Spend */}
              {cardData.length > 1 && (
                <div className="card">
                  <h2 className="text-lg font-semibold mb-4">By Card</h2>
                  <div className="space-y-2">
                    {cardData.map((cd) => {
                      const pct = (cd.value / (summary?.total_spend ?? 1)) * 100;
                      return (
                        <div key={cd.name}>
                          <div className="flex justify-between text-sm mb-1">
                            <span>{cd.name}</span>
                            <span className="font-medium">{formatINR(cd.value)}</span>
                          </div>
                          <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                            <div
                              className="h-full rounded-full"
                              style={{ width: `${pct}%`, backgroundColor: cd.color }}
                            />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Monthly Trends */}
          {trends && trends.length > 0 && (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Monthly Trends</h2>
              <ResponsiveContainer width="100%" height={250}>
                <BarChart data={trends}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
                  <XAxis dataKey="month" tick={{ fill: '#9CA3AF', fontSize: 12 }} />
                  <YAxis tick={{ fill: '#9CA3AF', fontSize: 12 }} />
                  <Tooltip
                    formatter={(value: number) => formatINR(value)}
                    contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                  />
                  <Bar dataKey="total" fill="#8B5CF6" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
