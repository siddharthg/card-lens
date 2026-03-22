import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import { formatINR } from '../types';
import type { CreditCard, Transaction, Statement } from '../types';

export default function StatementDetail() {
  const { id } = useParams<{ id: string }>();
  const [showText, setShowText] = useState(false);

  const { data: stmt } = useQuery<Statement>({
    queryKey: ['statement', id],
    queryFn: async () => {
      const stmts = await api.statements.list();
      return stmts.find((s) => s.id === id)!;
    },
    enabled: !!id,
  });

  const { data: cards } = useQuery<CreditCard[]>({
    queryKey: ['cards'],
    queryFn: () => api.cards.list(),
  });

  const { data: txns } = useQuery<Transaction[]>({
    queryKey: ['statement-txns', id],
    queryFn: () => api.statements.transactions(id!),
    enabled: !!id,
  });

  const { data: extractedText, isLoading: textLoading } = useQuery<string>({
    queryKey: ['statement-text', id],
    queryFn: () => api.statements.text(id!),
    enabled: !!id && showText,
  });

  const card = cards?.find((c) => c.id === stmt?.card_id);

  if (!id) return <p className="text-gray-500">Statement not found.</p>;

  const debits = txns?.filter((t) => t.amount > 0) ?? [];
  const credits = txns?.filter((t) => t.amount < 0) ?? [];
  const totalDebits = debits.reduce((s, t) => s + t.amount, 0);
  const totalCredits = credits.reduce((s, t) => s + Math.abs(t.amount), 0);

  return (
    <div className="h-[calc(100vh-4rem)]">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-xl font-bold">
            {card ? `${card.bank} ****${card.last_four}` : 'Statement'}
            {stmt?.period_start && stmt?.period_end && (
              <span className="text-gray-400 font-normal text-sm ml-3">
                {stmt.period_start} to {stmt.period_end}
              </span>
            )}
          </h1>
          {stmt && (
            <div className="flex gap-4 text-xs text-gray-400 mt-1">
              <span>Prev Dues: {formatINR(stmt.prev_balance)}</span>
              <span>Credits: {formatINR(stmt.payments_total)}</span>
              <span>Debits: {formatINR(stmt.purchase_total)}</span>
              <span className="font-semibold text-gray-300">Total: {formatINR(stmt.total_amount)}</span>
              <StatusBadge status={stmt.status} />
            </div>
          )}
          {stmt?.validation_message && (
            <p className={`text-xs mt-1 ${stmt.status === 'failed' ? 'text-red-400' : 'text-green-400'}`}>
              {stmt.validation_message}
            </p>
          )}
        </div>
        <button
          className={`btn-secondary text-sm ${showText ? 'bg-blue-600/20 text-blue-400' : ''}`}
          onClick={() => setShowText(!showText)}
        >
          {showText ? 'Hide Text' : 'Show Extracted Text'}
        </button>
      </div>

      {/* Split view */}
      <div className="flex gap-4 h-[calc(100%-5rem)]">
        {/* Left: PDF */}
        <div className="flex-1 min-w-0 rounded-lg overflow-hidden border border-gray-800">
          <iframe
            src={api.statements.pdfUrl(id)}
            className="w-full h-full"
            title="Statement PDF"
          />
        </div>

        {/* Right: Transactions or Text */}
        <div className="flex-1 min-w-0 flex flex-col">
          {showText ? (
            <div className="card flex-1 overflow-auto">
              <h2 className="text-sm font-semibold text-gray-400 mb-2">Extracted Text</h2>
              {textLoading ? (
                <p className="text-gray-500">Extracting...</p>
              ) : (
                <pre className="text-xs text-gray-300 whitespace-pre-wrap font-mono leading-5">
                  {extractedText}
                </pre>
              )}
            </div>
          ) : (
            <div className="card flex-1 overflow-auto">
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-sm font-semibold text-gray-400">
                  Transactions ({txns?.length ?? 0})
                </h2>
                <div className="flex gap-3 text-xs">
                  <span>Debits: {formatINR(totalDebits)}</span>
                  <span className="text-green-400">Credits: {formatINR(totalCredits)}</span>
                </div>
              </div>
              {!txns?.length ? (
                <p className="text-gray-500 text-sm">No transactions found.</p>
              ) : (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-left text-gray-500 border-b border-gray-800">
                      <th className="pb-2">Date</th>
                      <th className="pb-2">Description</th>
                      <th className="pb-2">Spender</th>
                      <th className="pb-2 text-right">Amount</th>
                    </tr>
                  </thead>
                  <tbody>
                    {txns.map((t) => (
                      <tr key={t.id} className="border-b border-gray-800/30 hover:bg-gray-800/20">
                        <td className="py-2 whitespace-nowrap text-gray-400">{t.txn_date}</td>
                        <td className="py-2 max-w-[250px] truncate" title={t.description}>
                          {t.merchant || t.description}
                        </td>
                        <td className="py-2 text-gray-400">{t.spender}</td>
                        <td className={`py-2 text-right font-mono ${t.amount < 0 ? 'text-green-400' : ''}`}>
                          {formatINR(t.amount)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    parsed: 'bg-green-900/30 text-green-400 border-green-800/50',
    failed: 'bg-red-900/30 text-red-400 border-red-800/50',
  };
  const style = styles[status] ?? 'bg-gray-800 text-gray-400 border-gray-700';
  return (
    <span className={`text-xs px-2 py-0.5 rounded border ${style}`}>
      {status}
    </span>
  );
}
