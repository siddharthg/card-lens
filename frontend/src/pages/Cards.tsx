import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { CreditCard, Statement } from '../types';
import { BANK_COLORS, formatINR } from '../types';
import StatementUpload from '../components/StatementUpload';

const BANKS = ['HDFC', 'ICICI', 'SBI', 'Amex', 'Axis', 'IDFC First', 'IndusInd', 'HSBC', 'Other'];

interface CardFormData {
  bank: string;
  card_name: string;
  last_four: string;
  billing_day: number;
  card_holder: string;
  addon_holders: string[];
  addon_holders_raw: string;
  stmt_password: string;
}

const emptyForm: CardFormData = {
  bank: 'HDFC',
  card_name: '',
  last_four: '',
  billing_day: 1,
  card_holder: '',
  addon_holders: [],
  addon_holders_raw: '',
  stmt_password: '',
};

export default function Cards() {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<CardFormData>(emptyForm);
  const [uploadCardId, setUploadCardId] = useState<string | null>(null);
  const [expandedCard, setExpandedCard] = useState<string | null>(null);

  const { data: cards, isLoading } = useQuery<CreditCard[]>({
    queryKey: ['cards'],
    queryFn: () => api.cards.list(),
  });

  const { data: statements } = useQuery<Statement[]>({
    queryKey: ['statements'],
    queryFn: () => api.statements.list(),
  });

  const createMutation = useMutation({
    mutationFn: (card: CardFormData) => api.cards.create(card),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cards'] });
      setShowForm(false);
      setForm(emptyForm);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, card }: { id: string; card: CardFormData }) => api.cards.update(id, card),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cards'] });
      setEditingId(null);
      setShowForm(false);
      setForm(emptyForm);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.cards.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['cards'] }),
  });

  const deleteStmtMutation = useMutation({
    mutationFn: (id: string) => api.statements.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['statements'] });
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
    },
  });

  const startEdit = (card: CreditCard) => {
    setEditingId(card.id);
    setForm({
      bank: card.bank,
      card_name: card.card_name,
      last_four: card.last_four,
      billing_day: card.billing_day,
      card_holder: card.card_holder,
      addon_holders: card.addon_holders,
      addon_holders_raw: card.addon_holders.join(', '),
      stmt_password: card.stmt_password || '',
    });
    setShowForm(true);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const submitForm = {
      ...form,
      addon_holders: form.addon_holders_raw
        ? form.addon_holders_raw.split(',').map((s) => s.trim()).filter(Boolean)
        : [],
    };
    if (editingId) {
      updateMutation.mutate({ id: editingId, card: submitForm });
    } else {
      createMutation.mutate(submitForm);
    }
  };

  const statementsForCard = (cardId: string) =>
    statements?.filter((s) => s.card_id === cardId) ?? [];

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Cards</h1>
        <button
          className="btn-primary"
          onClick={() => { setShowForm(true); setEditingId(null); setForm(emptyForm); }}
        >
          Add Card
        </button>
      </div>

      {/* Card Form */}
      {showForm && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4">{editingId ? 'Edit Card' : 'Add New Card'}</h2>
          <form onSubmit={handleSubmit} className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">Bank</label>
              <select className="select w-full" value={form.bank} onChange={(e) => setForm({ ...form, bank: e.target.value })}>
                {BANKS.map((b) => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Card Name</label>
              <input className="input w-full" placeholder="e.g. Regalia, Amazon Pay" value={form.card_name} onChange={(e) => setForm({ ...form, card_name: e.target.value })} required />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Last 4 Digits</label>
              <input className="input w-full" placeholder="1234" maxLength={4} pattern="\d{4}" value={form.last_four} onChange={(e) => setForm({ ...form, last_four: e.target.value })} required />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Billing Cycle Day</label>
              <input className="input w-full" type="number" min={1} max={31} value={form.billing_day} onChange={(e) => setForm({ ...form, billing_day: Number(e.target.value) })} />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Card Holder</label>
              <input className="input w-full" placeholder="Primary holder name" value={form.card_holder} onChange={(e) => setForm({ ...form, card_holder: e.target.value })} />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Statement Password (optional)</label>
              <input className="input w-full" placeholder="Override auto-generated password" value={form.stmt_password} onChange={(e) => setForm({ ...form, stmt_password: e.target.value })} />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-sm text-gray-400 mb-1">Add-on Card Holders (comma separated)</label>
              <input
                className="input w-full"
                placeholder="e.g. Spouse Name, Parent Name"
                value={form.addon_holders_raw}
                onChange={(e) => setForm({ ...form, addon_holders_raw: e.target.value })}
              />
            </div>
            <div className="sm:col-span-2 flex gap-3">
              <button type="submit" className="btn-primary">{editingId ? 'Update' : 'Add'} Card</button>
              <button type="button" className="btn-secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
            </div>
          </form>
        </div>
      )}

      {/* Card List */}
      {isLoading ? (
        <p className="text-gray-500">Loading...</p>
      ) : (cards?.length ?? 0) === 0 ? (
        <div className="card text-center py-12">
          <p className="text-gray-400 text-lg mb-2">No cards added yet</p>
          <p className="text-gray-500 text-sm mb-4">Add your credit cards to start tracking expenses</p>
          <button className="btn-primary" onClick={() => setShowForm(true)}>Add Your First Card</button>
        </div>
      ) : (
        <div className="space-y-4">
          {cards!.map((card) => {
            const color = BANK_COLORS[card.bank] ?? '#6B7280';
            const cardStatements = statementsForCard(card.id);
            const isExpanded = expandedCard === card.id;

            return (
              <div key={card.id} className="rounded-xl overflow-hidden border border-gray-800">
                {/* Card header */}
                <div
                  className="p-5 cursor-pointer"
                  style={{ background: `linear-gradient(135deg, ${color}20, ${color}08)` }}
                  onClick={() => setExpandedCard(isExpanded ? null : card.id)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-bold tracking-wider" style={{ color }}>{card.bank}</span>
                          <span className="text-sm font-medium">{card.card_name}</span>
                        </div>
                        <p className="font-mono text-gray-400 text-sm mt-1">
                          •••• •••• •••• {card.last_four}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-4">
                      <div className="text-right text-sm text-gray-400">
                        <p>{card.card_holder || 'No holder set'}</p>
                        <p className="text-xs">Billing: {card.billing_day}th &middot; {cardStatements.length} statements</p>
                      </div>
                      <span className={`transition-transform ${isExpanded ? 'rotate-180' : ''}`}>▾</span>
                    </div>
                  </div>
                </div>

                {/* Expanded details */}
                {isExpanded && (
                  <div className="bg-gray-900 border-t border-gray-800 p-5">
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                      {/* Upload section */}
                      <div>
                        <h3 className="text-sm font-semibold text-gray-400 mb-3 uppercase tracking-wider">Upload Statement</h3>
                        {uploadCardId === card.id ? (
                          <StatementUpload
                            cardId={card.id}
                            onSuccess={() => setUploadCardId(null)}
                          />
                        ) : (
                          <button
                            className="btn-secondary w-full"
                            onClick={() => setUploadCardId(card.id)}
                          >
                            Upload PDF Statement
                          </button>
                        )}
                      </div>

                      {/* Statement history */}
                      <div>
                        <h3 className="text-sm font-semibold text-gray-400 mb-3 uppercase tracking-wider">Statement History</h3>
                        {cardStatements.length === 0 ? (
                          <p className="text-gray-500 text-sm">No statements uploaded yet</p>
                        ) : (
                          <div className="space-y-2 max-h-48 overflow-y-auto">
                            {cardStatements.map((stmt) => (
                              <div key={stmt.id} className="flex items-center justify-between bg-gray-800 rounded-lg px-3 py-2">
                                <div>
                                  <p className="text-sm font-medium">{stmt.filename}</p>
                                  <p className="text-xs text-gray-500">
                                    {stmt.period_start && stmt.period_end
                                      ? `${stmt.period_start} to ${stmt.period_end}`
                                      : new Date(stmt.parsed_at).toLocaleDateString()}
                                    {stmt.total_amount > 0 && ` — ${formatINR(stmt.total_amount)}`}
                                  </p>
                                </div>
                                <div className="flex items-center gap-2">
                                  <span className={`text-xs px-2 py-0.5 rounded ${
                                    stmt.status === 'parsed' ? 'bg-green-900 text-green-300' :
                                    stmt.status === 'failed' ? 'bg-red-900 text-red-300' :
                                    'bg-yellow-900 text-yellow-300'
                                  }`}>
                                    {stmt.status}
                                  </span>
                                  <button
                                    className="text-xs text-red-400 hover:text-red-300"
                                    onClick={() => { if (confirm('Delete this statement and its transactions?')) deleteStmtMutation.mutate(stmt.id); }}
                                  >
                                    ✕
                                  </button>
                                </div>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>

                    {/* Card actions */}
                    <div className="flex gap-3 mt-4 pt-4 border-t border-gray-800">
                      <button className="btn-secondary text-sm" onClick={() => startEdit(card)}>Edit Card</button>
                      <button
                        className="btn-danger text-sm"
                        onClick={() => { if (confirm('Delete this card and all its transactions?')) deleteMutation.mutate(card.id); }}
                      >
                        Delete Card
                      </button>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
