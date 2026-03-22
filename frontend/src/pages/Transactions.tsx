import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { formatINR } from '../types';
import type { CreditCard, Transaction, TransactionListResult, Statement } from '../types';

interface EditState {
  category: string;
  sub_category: string;
  spender: string;
  tags: string;
  notes: string;
}

export default function Transactions() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState({
    card_id: '',
    category: '',
    from: '',
    to: '',
    q: '',
  });
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [editState, setEditState] = useState<EditState | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const { data: cards } = useQuery<CreditCard[]>({
    queryKey: ['cards'],
    queryFn: () => api.cards.list(),
  });

  const { data: categories } = useQuery<Record<string, string[]>>({
    queryKey: ['categories'],
    queryFn: () => api.categories.list(),
  });

  const { data: statements } = useQuery<Statement[]>({
    queryKey: ['statements'],
    queryFn: () => api.statements.list(),
  });

  const { data, isLoading } = useQuery<TransactionListResult>({
    queryKey: ['transactions', page, filters],
    queryFn: () => api.transactions.list({ ...filters, page, limit: 50 }),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, updates }: { id: string; updates: Partial<Transaction> }) =>
      api.transactions.update(id, updates),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
      setExpandedId(null);
      setEditState(null);
    },
  });

  const bulkMutation = useMutation({
    mutationFn: ({ ids, category }: { ids: string[]; category: string }) =>
      api.transactions.bulkUpdate(ids, { category }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
      setSelected(new Set());
    },
  });

  const cardMap = new Map(cards?.map((c) => [c.id, c]) ?? []);
  const stmtMap = new Map(statements?.map((s) => [s.id, s]) ?? []);
  const txns = data?.transactions ?? [];
  const totalPages = Math.ceil((data?.total ?? 0) / (data?.limit ?? 50));

  // Collect all unique spenders for suggestions
  const allSpenders = [...new Set(txns.map((t) => t.spender).filter(Boolean))];

  const toggleSelect = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelected(next);
  };

  const toggleAll = () => {
    if (selected.size === txns.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(txns.map((t) => t.id)));
    }
  };

  const openEdit = (t: Transaction) => {
    if (expandedId === t.id) {
      setExpandedId(null);
      setEditState(null);
    } else {
      setExpandedId(t.id);
      setEditState({
        category: t.category,
        sub_category: t.sub_category,
        spender: t.spender,
        tags: t.tags.join(', '),
        notes: t.notes,
      });
    }
  };

  const saveEdit = (id: string) => {
    if (!editState) return;
    const tags = editState.tags
      ? editState.tags.split(',').map((s) => s.trim()).filter(Boolean)
      : [];
    updateMutation.mutate({
      id,
      updates: {
        category: editState.category,
        sub_category: editState.sub_category,
        spender: editState.spender,
        tags,
        notes: editState.notes,
      },
    });
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Transactions</h1>
        <a
          href={api.transactions.export(filters)}
          className="btn-secondary text-sm"
          download
        >
          Export CSV
        </a>
      </div>

      {/* Filters */}
      <div className="card mb-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
          <input
            type="text"
            placeholder="Search..."
            className="input"
            value={filters.q}
            onChange={(e) => { setFilters({ ...filters, q: e.target.value }); setPage(1); }}
          />
          <select
            className="select"
            value={filters.card_id}
            onChange={(e) => { setFilters({ ...filters, card_id: e.target.value }); setPage(1); }}
          >
            <option value="">All Cards</option>
            {cards?.map((c) => (
              <option key={c.id} value={c.id}>{c.bank} {c.card_name}</option>
            ))}
          </select>
          <select
            className="select"
            value={filters.category}
            onChange={(e) => { setFilters({ ...filters, category: e.target.value }); setPage(1); }}
          >
            <option value="">All Categories</option>
            {Object.keys(categories ?? {}).map((c) => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
          <input
            type="date"
            className="input"
            value={filters.from}
            onChange={(e) => { setFilters({ ...filters, from: e.target.value }); setPage(1); }}
          />
          <input
            type="date"
            className="input"
            value={filters.to}
            onChange={(e) => { setFilters({ ...filters, to: e.target.value }); setPage(1); }}
          />
        </div>
      </div>

      {/* Bulk actions */}
      {selected.size > 0 && (
        <div className="card mb-4 flex items-center gap-3">
          <span className="text-sm text-gray-400">{selected.size} selected</span>
          <select
            className="select text-sm"
            defaultValue=""
            onChange={(e) => {
              if (e.target.value) {
                bulkMutation.mutate({ ids: [...selected], category: e.target.value });
              }
            }}
          >
            <option value="" disabled>Bulk categorize...</option>
            {Object.keys(categories ?? {}).map((c) => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
        </div>
      )}

      {/* Table */}
      <div className="card overflow-x-auto">
        {isLoading ? (
          <p className="text-gray-500">Loading...</p>
        ) : txns.length === 0 ? (
          <p className="text-gray-500">No transactions found. Upload a credit card statement to get started.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-400 border-b border-gray-800">
                <th className="pb-3 pr-2">
                  <input type="checkbox" checked={selected.size === txns.length && txns.length > 0} onChange={toggleAll} />
                </th>
                <th className="pb-3">Date</th>
                <th className="pb-3">Merchant</th>
                <th className="pb-3">Category</th>
                <th className="pb-3">Spender</th>
                <th className="pb-3">Card</th>
                <th className="pb-3 text-right">Amount</th>
              </tr>
            </thead>
            <tbody>
              {txns.map((t) => {
                const card = cardMap.get(t.card_id);
                const isExpanded = expandedId === t.id;
                return (
                  <tr
                    key={t.id}
                    className={`border-b border-gray-800/50 ${isExpanded ? 'bg-gray-800/40' : 'hover:bg-gray-800/30'}`}
                  >
                    {/* Main row - use a single td with a nested layout when expanded */}
                    {isExpanded ? (
                      <td colSpan={7} className="py-0">
                        {/* Condensed row header */}
                        <div
                          className="flex items-center gap-4 py-3 cursor-pointer"
                          onClick={() => openEdit(t)}
                        >
                          <input
                            type="checkbox"
                            checked={selected.has(t.id)}
                            onChange={(e) => { e.stopPropagation(); toggleSelect(t.id); }}
                            onClick={(e) => e.stopPropagation()}
                          />
                          <span className="whitespace-nowrap text-gray-400">{t.txn_date}</span>
                          <span className="font-medium flex-1">{t.merchant || t.description}</span>
                          <span className={`font-mono ${t.amount < 0 ? 'text-green-400' : ''}`}>
                            {formatINR(t.amount)}
                          </span>
                          <span className="text-xs text-blue-400">Close</span>
                        </div>

                        {/* Edit panel */}
                        {editState && (
                          <div className="pb-4 px-8 border-t border-gray-700/50">
                            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mt-3">
                              {/* Category */}
                              <div>
                                <label className="block text-xs text-gray-400 mb-1">Category</label>
                                <select
                                  className="select w-full text-sm"
                                  value={editState.category}
                                  onChange={(e) => setEditState({ ...editState, category: e.target.value })}
                                >
                                  {Object.keys(categories ?? {}).map((c) => (
                                    <option key={c} value={c}>{c}</option>
                                  ))}
                                </select>
                              </div>

                              {/* Sub Category */}
                              <div>
                                <label className="block text-xs text-gray-400 mb-1">Sub Category</label>
                                <select
                                  className="select w-full text-sm"
                                  value={editState.sub_category}
                                  onChange={(e) => setEditState({ ...editState, sub_category: e.target.value })}
                                >
                                  <option value="">None</option>
                                  {(categories?.[editState.category] ?? []).map((sc) => (
                                    <option key={sc} value={sc}>{sc}</option>
                                  ))}
                                </select>
                              </div>

                              {/* Spender */}
                              <div>
                                <label className="block text-xs text-gray-400 mb-1">Spender</label>
                                <input
                                  className="input w-full text-sm"
                                  placeholder="Who made this purchase?"
                                  value={editState.spender}
                                  onChange={(e) => setEditState({ ...editState, spender: e.target.value })}
                                  list="spender-suggestions"
                                />
                                <datalist id="spender-suggestions">
                                  {allSpenders.map((s) => (
                                    <option key={s} value={s} />
                                  ))}
                                  {/* Also suggest card holders */}
                                  {card?.card_holder && <option value={card.card_holder} />}
                                  {card?.addon_holders?.map((h) => (
                                    <option key={h} value={h} />
                                  ))}
                                </datalist>
                              </div>

                              {/* Tags */}
                              <div>
                                <label className="block text-xs text-gray-400 mb-1">Tags (comma separated)</label>
                                <input
                                  className="input w-full text-sm"
                                  placeholder="e.g. reimbursable, business"
                                  value={editState.tags}
                                  onChange={(e) => setEditState({ ...editState, tags: e.target.value })}
                                />
                              </div>
                            </div>

                            {/* Notes */}
                            <div className="mt-3">
                              <label className="block text-xs text-gray-400 mb-1">Notes</label>
                              <textarea
                                className="input w-full text-sm resize-none"
                                rows={2}
                                placeholder="Add a note..."
                                value={editState.notes}
                                onChange={(e) => setEditState({ ...editState, notes: e.target.value })}
                              />
                            </div>

                            {/* Statement info */}
                            {t.statement_id && (() => {
                              const stmt = stmtMap.get(t.statement_id);
                              const stmtCard = stmt ? cardMap.get(stmt.card_id) : null;
                              return stmt ? (
                                <div className="mt-3 flex items-center gap-3 text-xs bg-gray-800/60 rounded px-3 py-2">
                                  <span className="text-gray-400">Statement:</span>
                                  <span className="text-gray-300">{stmt.filename}</span>
                                  {stmt.period_start && stmt.period_end && (
                                    <span className="text-gray-500">
                                      {stmt.period_start} to {stmt.period_end}
                                    </span>
                                  )}
                                  {stmt.total_amount > 0 && (
                                    <span className="text-gray-500">
                                      Total: {formatINR(stmt.total_amount)}
                                    </span>
                                  )}
                                  {stmtCard && (
                                    <span className="text-gray-500">
                                      {stmtCard.bank} ****{stmtCard.last_four}
                                    </span>
                                  )}
                                </div>
                              ) : null;
                            })()}

                            {/* Description + meta info */}
                            <div className="mt-3 text-xs text-gray-500">
                              <span>Raw: {t.description}</span>
                              {t.company && <span className="ml-3">Company: {t.company}</span>}
                              {t.is_recurring && <span className="ml-3 text-yellow-500">Recurring</span>}
                              {t.is_international && <span className="ml-3 text-blue-400">International</span>}
                              {t.tags.length > 0 && (
                                <span className="ml-3">
                                  Current tags: {t.tags.map((tag) => (
                                    <span key={tag} className="inline-block bg-gray-700 rounded px-1.5 py-0.5 mr-1">{tag}</span>
                                  ))}
                                </span>
                              )}
                            </div>

                            {/* Actions */}
                            <div className="flex gap-2 mt-3">
                              <button
                                className="btn-primary text-sm"
                                onClick={() => saveEdit(t.id)}
                                disabled={updateMutation.isPending}
                              >
                                {updateMutation.isPending ? 'Saving...' : 'Save'}
                              </button>
                              <button
                                className="btn-secondary text-sm"
                                onClick={() => { setExpandedId(null); setEditState(null); }}
                              >
                                Cancel
                              </button>
                            </div>
                          </div>
                        )}
                      </td>
                    ) : (
                      <>
                        <td className="py-3 pr-2">
                          <input type="checkbox" checked={selected.has(t.id)} onChange={() => toggleSelect(t.id)} />
                        </td>
                        <td className="py-3 whitespace-nowrap">{t.txn_date}</td>
                        <td className="py-3">
                          <button
                            className="text-left w-full"
                            onClick={() => openEdit(t)}
                          >
                            <div className="font-medium hover:text-blue-400 transition-colors">
                              {t.merchant || t.description}
                            </div>
                            {t.merchant && (
                              <div className="text-xs text-gray-500 truncate max-w-xs">{t.description}</div>
                            )}
                          </button>
                        </td>
                        <td className="py-3">
                          <span className="text-xs px-2 py-1 rounded bg-gray-800">
                            {t.category}
                          </span>
                          {t.sub_category && (
                            <span className="text-xs text-gray-500 ml-1">{t.sub_category}</span>
                          )}
                        </td>
                        <td className="py-3 text-xs text-gray-400">
                          {t.spender || (
                            <button
                              className="text-gray-600 hover:text-gray-400 italic"
                              onClick={() => openEdit(t)}
                            >
                              assign
                            </button>
                          )}
                        </td>
                        <td className="py-3 text-xs text-gray-400">
                          {card ? `${card.bank} ****${card.last_four}` : t.card_id.slice(0, 8)}
                        </td>
                        <td className="py-3 text-right font-mono">
                          <span className={t.amount < 0 ? 'text-green-400' : ''}>
                            {formatINR(t.amount)}
                          </span>
                        </td>
                      </>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between mt-4 pt-4 border-t border-gray-800">
            <p className="text-sm text-gray-400">
              Page {page} of {totalPages} ({data?.total} total)
            </p>
            <div className="flex gap-2">
              <button
                className="btn-secondary text-sm"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
              >
                Previous
              </button>
              <button
                className="btn-secondary text-sm"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
