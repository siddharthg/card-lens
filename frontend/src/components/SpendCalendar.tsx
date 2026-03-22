import { useMemo } from 'react';
import { formatINR } from '../types';
import {
  startOfYear, endOfYear, eachDayOfInterval, format, getDay, getWeek,
  startOfWeek, differenceInWeeks,
} from 'date-fns';

interface Props {
  data: Record<string, number>;
  year: number;
}

const MONTH_LABELS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

function getColor(amount: number, max: number): string {
  if (amount === 0) return '#1a1a2e';
  const intensity = Math.min(amount / max, 1);
  if (intensity < 0.25) return '#064e3b';
  if (intensity < 0.5) return '#065f46';
  if (intensity < 0.75) return '#b45309';
  return '#dc2626';
}

export default function SpendCalendar({ data, year }: Props) {
  const { cells, maxAmount, weekCount } = useMemo(() => {
    const yearStart = startOfYear(new Date(year, 0, 1));
    const yearEnd = endOfYear(new Date(year, 0, 1));
    const days = eachDayOfInterval({ start: yearStart, end: yearEnd });

    let max = 0;
    const vals = days.map((d) => {
      const key = format(d, 'yyyy-MM-dd');
      const amount = data[key] || 0;
      if (amount > max) max = amount;
      return { date: d, key, amount };
    });

    const firstWeekStart = startOfWeek(yearStart, { weekStartsOn: 0 });
    const weeks = differenceInWeeks(yearEnd, firstWeekStart) + 1;

    return { cells: vals, maxAmount: max || 1, weekCount: weeks };
  }, [data, year]);

  // Build grid: 7 rows (days) x N weeks (columns)
  const grid: (typeof cells[0] | null)[][] = Array.from({ length: 7 }, () =>
    Array.from({ length: weekCount }, () => null)
  );

  const yearStart = startOfYear(new Date(year, 0, 1));
  const firstWeekStart = startOfWeek(yearStart, { weekStartsOn: 0 });

  for (const cell of cells) {
    const dayOfWeek = getDay(cell.date); // 0=Sun
    const weekIdx = differenceInWeeks(
      startOfWeek(cell.date, { weekStartsOn: 0 }),
      firstWeekStart
    );
    if (weekIdx >= 0 && weekIdx < weekCount) {
      grid[dayOfWeek][weekIdx] = cell;
    }
  }

  return (
    <div>
      {/* Month labels */}
      <div className="flex mb-1 ml-8">
        {MONTH_LABELS.map((m, i) => (
          <div
            key={m}
            className="text-xs text-gray-500"
            style={{ width: `${100 / 12}%` }}
          >
            {m}
          </div>
        ))}
      </div>

      <div className="flex gap-0.5">
        {/* Day labels */}
        <div className="flex flex-col gap-0.5 mr-1">
          {['', 'Mon', '', 'Wed', '', 'Fri', ''].map((d, i) => (
            <div key={i} className="text-xs text-gray-500 h-3 leading-3">{d}</div>
          ))}
        </div>

        {/* Grid */}
        <div className="flex gap-0.5 flex-1 overflow-x-auto">
          {Array.from({ length: weekCount }, (_, weekIdx) => (
            <div key={weekIdx} className="flex flex-col gap-0.5">
              {Array.from({ length: 7 }, (_, dayIdx) => {
                const cell = grid[dayIdx]?.[weekIdx];
                return (
                  <div
                    key={dayIdx}
                    className="w-3 h-3 rounded-sm cursor-pointer"
                    style={{ backgroundColor: cell ? getColor(cell.amount, maxAmount) : '#111' }}
                    title={cell ? `${cell.key}: ${formatINR(cell.amount)}` : ''}
                  />
                );
              })}
            </div>
          ))}
        </div>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-2 mt-3 text-xs text-gray-500">
        <span>Less</span>
        {[0, 0.25, 0.5, 0.75, 1].map((intensity) => (
          <div
            key={intensity}
            className="w-3 h-3 rounded-sm"
            style={{ backgroundColor: getColor(intensity * maxAmount, maxAmount) }}
          />
        ))}
        <span>More</span>
      </div>
    </div>
  );
}
