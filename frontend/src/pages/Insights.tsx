import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from 'recharts';
import { api } from '../api/client';
import { formatINR } from '../types';
import SpendCalendar from '../components/SpendCalendar';
import type { MonthTrend, Transaction } from '../types';

interface RecurringGroup {
  merchant: string;
  category: string;
  avg_amount: number;
  count: number;
  transactions: Transaction[];
}

export default function Insights() {
  const currentYear = new Date().getFullYear();
  const [calendarYear, setCalendarYear] = useState(currentYear);

  const { data: trends } = useQuery<MonthTrend[]>({
    queryKey: ['trends', 12],
    queryFn: () => api.analytics.trends({ months: 12 }),
  });

  const { data: calendarData } = useQuery<Record<string, number>>({
    queryKey: ['calendar', calendarYear],
    queryFn: () => api.analytics.calendar({ year: calendarYear }),
  });

  const { data: recurring } = useQuery<RecurringGroup[]>({
    queryKey: ['recurring'],
    queryFn: async () => {
      const res = await fetch('/api/analytics/recurring');
      return res.json();
    },
  });

  // Calculate category trends from monthly data
  const { data: categoryTrends } = useQuery({
    queryKey: ['category-trends'],
    queryFn: async () => {
      const months = 6;
      const now = new Date();
      const results: Record<string, { month: string; [key: string]: string | number }[]> = {};

      for (let i = months - 1; i >= 0; i--) {
        const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
        const month = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
        const summary = await api.analytics.summary({ date: month });

        for (const [cat, amount] of Object.entries(summary.by_category)) {
          if (!results[cat]) results[cat] = [];
          results[cat].push({ month, amount });
        }
      }

      // Get top 5 categories by total spend
      const totals = Object.entries(results)
        .map(([cat, data]) => ({
          category: cat,
          total: data.reduce((sum, d) => sum + (d.amount as number), 0),
          data,
        }))
        .sort((a, b) => b.total - a.total)
        .slice(0, 5);

      return totals;
    },
  });

  const COLORS = ['#3B82F6', '#EF4444', '#10B981', '#F59E0B', '#8B5CF6'];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Insights</h1>

      {/* Spend Calendar Heat Map */}
      <div className="card mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Spending Heat Map</h2>
          <div className="flex gap-2">
            <button
              className="btn-secondary text-sm"
              onClick={() => setCalendarYear(calendarYear - 1)}
            >
              {calendarYear - 1}
            </button>
            <span className="px-3 py-1.5 text-sm font-medium">{calendarYear}</span>
            {calendarYear < currentYear && (
              <button
                className="btn-secondary text-sm"
                onClick={() => setCalendarYear(calendarYear + 1)}
              >
                {calendarYear + 1}
              </button>
            )}
          </div>
        </div>
        {calendarData ? (
          <SpendCalendar data={calendarData} year={calendarYear} />
        ) : (
          <p className="text-gray-500 text-sm">Loading...</p>
        )}
      </div>

      {/* Monthly Trend Line */}
      {trends && trends.length > 0 && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4">Monthly Spending Trend</h2>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={trends}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis dataKey="month" tick={{ fill: '#9CA3AF', fontSize: 12 }} />
              <YAxis tick={{ fill: '#9CA3AF', fontSize: 12 }} />
              <Tooltip
                formatter={(value: number) => formatINR(value)}
                contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                labelStyle={{ color: '#9CA3AF' }}
              />
              <Line
                type="monotone"
                dataKey="total"
                stroke="#3B82F6"
                strokeWidth={2}
                dot={{ fill: '#3B82F6', r: 4 }}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Category Trends */}
      {categoryTrends && categoryTrends.length > 0 && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4">Category Trends (Last 6 Months)</h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis
                dataKey="month"
                tick={{ fill: '#9CA3AF', fontSize: 12 }}
                type="category"
                allowDuplicatedCategory={false}
              />
              <YAxis tick={{ fill: '#9CA3AF', fontSize: 12 }} />
              <Tooltip
                formatter={(value: number) => formatINR(value)}
                contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                labelStyle={{ color: '#9CA3AF' }}
              />
              {categoryTrends.map((ct, i) => (
                <Line
                  key={ct.category}
                  data={ct.data}
                  type="monotone"
                  dataKey="amount"
                  name={ct.category}
                  stroke={COLORS[i]}
                  strokeWidth={2}
                  dot={{ r: 3 }}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
          <div className="flex flex-wrap gap-4 mt-3">
            {categoryTrends.map((ct, i) => (
              <div key={ct.category} className="flex items-center gap-2 text-sm">
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: COLORS[i] }} />
                <span className="text-gray-400">{ct.category}</span>
                <span className="font-medium">{formatINR(ct.total)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recurring Subscriptions */}
      <div className="card">
        <h2 className="text-lg font-semibold mb-4">Detected Recurring Charges</h2>
        {(!recurring || recurring.length === 0) ? (
          <p className="text-gray-500 text-sm">
            No recurring charges detected yet. Recurring patterns are identified after 3+ monthly occurrences.
          </p>
        ) : (
          <div className="space-y-3">
            {recurring.map((g, i) => (
              <div key={i} className="flex items-center justify-between bg-gray-800 rounded-lg px-4 py-3">
                <div>
                  <p className="font-medium">{g.merchant}</p>
                  <p className="text-xs text-gray-500">{g.category} &middot; {g.count} charges</p>
                </div>
                <div className="text-right">
                  <p className="font-semibold">{formatINR(g.avg_amount)}</p>
                  <p className="text-xs text-gray-500">avg/month</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
