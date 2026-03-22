import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../api/client';
import { formatINR } from '../types';
import type { CreditCard, Statement } from '../types';
import StatementUpload from '../components/StatementUpload';

export default function Statements() {
  const [searchParams, setSearchParams] = useSearchParams();
  const filter = searchParams.get('filter') || 'all';
  const setFilter = (v: string) => {
    if (v === 'all') {
      searchParams.delete('filter');
    } else {
      searchParams.set('filter', v);
    }
    setSearchParams(searchParams, { replace: true });
  };
  const [showUpload, setShowUpload] = useState(false);

  const { data: statements, isLoading } = useQuery<Statement[]>({
    queryKey: ['statements'],
    queryFn: () => api.statements.list(),
  });

  const { data: cards } = useQuery<CreditCard[]>({
    queryKey: ['cards'],
    queryFn: () => api.cards.list(),
  });

  const cardMap = new Map(cards?.map((c) => [c.id, c]) ?? []);

  // Build filter options: group cards by bank
  const banks = [...new Set(cards?.map((c) => c.bank) ?? [])].sort();

  const filtered = statements?.filter((s) => {
    if (filter === 'all') return true;
    const card = cardMap.get(s.card_id);
    if (!card) return false;
    // filter can be "bank:HDFC" or "card:<id>"
    if (filter.startsWith('bank:')) return card.bank === filter.slice(5);
    if (filter.startsWith('card:')) return s.card_id === filter.slice(5);
    return true;
  })
  .sort((a, b) => (b.period_start ?? '').localeCompare(a.period_start ?? '')) ?? [];

  const parsed = filtered.filter((s) => s.status === 'parsed');
  const failed = filtered.filter((s) => s.status === 'failed');

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Statements</h1>
        <div className="flex items-center gap-3">
          <button
            className="btn-primary text-sm"
            onClick={() => setShowUpload(!showUpload)}
          >
            {showUpload ? 'Hide Upload' : 'Upload Statements'}
          </button>
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
          >
            <option value="all">All Cards</option>
            {banks.map((bank) => {
              const bankCards = cards?.filter((c) => c.bank === bank) ?? [];
              return [
                <option key={`bank:${bank}`} value={`bank:${bank}`}>
                  {bank} (all)
                </option>,
                ...bankCards.map((c) => (
                  <option key={`card:${c.id}`} value={`card:${c.id}`}>
                    &nbsp;&nbsp;{bank} ****{c.last_four}
                  </option>
                )),
              ];
            })}
          </select>
        </div>
      </div>

      {/* Upload section */}
      {showUpload && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-3">Upload Statements</h2>
          <p className="text-sm text-gray-400 mb-4">
            Select PDF files to upload. The correct card is auto-detected from each statement.
          </p>
          <StatementUpload onSuccess={() => {}} />
        </div>
      )}

      {/* Summary stats */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <div className="card">
          <p className="text-sm text-gray-400">Total Statements</p>
          <p className="text-2xl font-bold">{filtered.length}</p>
        </div>
        <div className="card">
          <p className="text-sm text-green-400">Parsed OK</p>
          <p className="text-2xl font-bold text-green-400">{parsed.length}</p>
        </div>
        <div className="card">
          <p className="text-sm text-red-400">Parse Issues</p>
          <p className="text-2xl font-bold text-red-400">{failed.length}</p>
        </div>
      </div>

      {/* Failed statements section */}
      {failed.length > 0 && (
        <div className="mb-6">
          <h2 className="text-lg font-semibold text-red-400 mb-3">Statements with Issues</h2>
          <div className="space-y-2">
            {failed.map((s) => (
              <StatementRow key={s.id} stmt={s} card={cardMap.get(s.card_id)} />
            ))}
          </div>
        </div>
      )}

      {/* All statements */}
      <h2 className="text-lg font-semibold mb-3">All Statements</h2>
      <div className="card overflow-x-auto">
        {isLoading ? (
          <p className="text-gray-500">Loading...</p>
        ) : !filtered.length ? (
          <p className="text-gray-500">
            {filter === 'all' ? 'No statements found. Sync from Gmail or upload a PDF.' : 'No statements for this filter.'}
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-400 border-b border-gray-800">
                <th className="pb-3">Card</th>
                <th className="pb-3">Period</th>
                <th className="pb-3 text-center">Txns</th>
                <th className="pb-3 text-right">Prev Dues</th>
                <th className="pb-3 text-right">Credits</th>
                <th className="pb-3 text-right">Debits</th>
                <th className="pb-3 text-right">Total Dues</th>
                <th className="pb-3 text-center">Status</th>
                <th className="pb-3">Validation</th>
                <th className="pb-3 text-center">PDF</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((s) => {
                const card = cardMap.get(s.card_id);
                const isFailed = s.status === 'failed';
                return (
                  <tr
                    key={s.id}
                    className={`border-b border-gray-800/50 ${isFailed ? 'bg-red-900/10' : 'hover:bg-gray-800/30'}`}
                  >
                    <td className="py-3 text-xs">
                      {card ? (
                        <span>
                          <span className="font-medium">{card.bank}</span>
                          <span className="text-gray-500 ml-1">****{card.last_four}</span>
                        </span>
                      ) : (
                        <span className="text-gray-500">{s.card_id.slice(0, 8)}</span>
                      )}
                    </td>
                    <td className="py-3 whitespace-nowrap text-xs" title={s.filename}>
                      {s.period_start && s.period_end ? (
                        <span>{s.period_start} to {s.period_end}</span>
                      ) : (
                        <span className="text-gray-500">-</span>
                      )}
                    </td>
                    <td className="py-3 text-center font-mono">
                      {s.txn_count}
                    </td>
                    <td className="py-3 text-right font-mono text-xs">
                      {s.prev_balance > 0 ? formatINR(s.prev_balance) : '-'}
                    </td>
                    <td className="py-3 text-right font-mono text-xs text-green-400">
                      {s.payments_total > 0 ? formatINR(s.payments_total) : '-'}
                    </td>
                    <td className="py-3 text-right font-mono text-xs">
                      {s.purchase_total > 0 ? formatINR(s.purchase_total) : '-'}
                    </td>
                    <td className="py-3 text-right font-mono text-xs font-semibold">
                      {s.total_amount > 0 ? formatINR(s.total_amount) : '-'}
                    </td>
                    <td className="py-3 text-center">
                      <StatusBadge status={s.status} />
                    </td>
                    <td className="py-3 text-xs max-w-[300px]">
                      {s.validation_message ? (
                        <span
                          className={`${isFailed ? 'text-red-400' : 'text-gray-500'}`}
                          title={s.validation_message}
                        >
                          {truncate(s.validation_message, 80)}
                        </span>
                      ) : (
                        <span className="text-gray-600">-</span>
                      )}
                    </td>
                    <td className="py-3 text-center">
                      <Link
                        to={`/statements/${s.id}`}
                        className="text-blue-400 hover:text-blue-300 text-xs"
                      >
                        View
                      </Link>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function StatementRow({ stmt, card }: { stmt: Statement; card?: CreditCard }) {
  return (
    <div className="card border border-red-800/30 bg-red-900/10">
      <div className="flex items-center justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <StatusBadge status={stmt.status} />
            <span className="text-sm font-medium">
              {card ? `${card.bank} ****${card.last_four}` : stmt.card_id.slice(0, 8)}
            </span>
            {stmt.period_start && stmt.period_end && (
              <span className="text-xs text-gray-500">{stmt.period_start} to {stmt.period_end}</span>
            )}
          </div>
          <p className="text-xs text-gray-400 truncate">{stmt.filename}</p>
          {stmt.validation_message && (
            <p className="text-xs text-red-400 mt-1">{stmt.validation_message}</p>
          )}
        </div>
        <div className="flex items-center gap-3 text-sm">
          <span className="font-mono">{stmt.txn_count} txns</span>
          {stmt.total_amount > 0 && (
            <span className="font-mono">{formatINR(stmt.total_amount)}</span>
          )}
          <Link
            to={`/statements/${stmt.id}`}
            className="btn-secondary text-xs"
          >
            View
          </Link>
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    parsed: 'bg-green-900/30 text-green-400 border-green-800/50',
    failed: 'bg-red-900/30 text-red-400 border-red-800/50',
    pending: 'bg-yellow-900/30 text-yellow-400 border-yellow-800/50',
  };
  const style = styles[status] ?? 'bg-gray-800 text-gray-400 border-gray-700';
  return (
    <span className={`text-xs px-2 py-0.5 rounded border ${style}`}>
      {status}
    </span>
  );
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '...' : s;
}
