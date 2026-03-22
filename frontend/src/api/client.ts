import type {
  CreditCard,
  Transaction,
  TransactionListResult,
  Statement,
  SpendSummary,
  OAuthAccount,
  MonthTrend,
  SyncError,
} from '../types';

const BASE = '/api';

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    let msg = text;
    try {
      msg = JSON.parse(text).error || text;
    } catch {
      // use raw text
    }
    throw new ApiError(res.status, msg);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

function qs(params: Record<string, string | number | undefined>): string {
  const entries = Object.entries(params).filter(([, v]) => v !== undefined && v !== '');
  return new URLSearchParams(entries.map(([k, v]) => [k, String(v)])).toString();
}

export const api = {
  // Cards
  cards: {
    list: () => request<CreditCard[]>('/cards'),
    get: (id: string) => request<CreditCard>(`/cards/${id}`),
    create: (card: Partial<CreditCard>) =>
      request<CreditCard>('/cards', { method: 'POST', body: JSON.stringify(card) }),
    update: (id: string, card: Partial<CreditCard>) =>
      request<CreditCard>(`/cards/${id}`, { method: 'PUT', body: JSON.stringify(card) }),
    delete: (id: string) =>
      request<void>(`/cards/${id}`, { method: 'DELETE' }),
  },

  // Transactions
  transactions: {
    list: (params: Record<string, string | number | undefined> = {}) =>
      request<TransactionListResult>(`/transactions?${qs(params)}`),
    update: (id: string, txn: Partial<Transaction>) =>
      request<Transaction>(`/transactions/${id}`, { method: 'PUT', body: JSON.stringify(txn) }),
    bulkUpdate: (ids: string[], updates: { category?: string; sub_category?: string; spender?: string }) =>
      request<{ updated: number }>('/transactions/bulk', {
        method: 'PUT',
        body: JSON.stringify({ ids, ...updates }),
      }),
    export: (params: Record<string, string | number | undefined> = {}) =>
      `${BASE}/transactions/export?${qs(params)}`,
  },

  // Statements
  statements: {
    list: (cardId?: string) =>
      request<Statement[]>(`/statements${cardId ? `?card_id=${cardId}` : ''}`),
    pdfUrl: (id: string) => `${BASE}/statements/${id}/pdf`,
    transactions: (id: string) => request<Transaction[]>(`/statements/${id}/transactions`),
    text: async (id: string) => {
      const res = await fetch(`${BASE}/statements/${id}/text`);
      if (!res.ok) throw new ApiError(res.status, await res.text());
      return res.text();
    },
    upload: async (file: File, cardId: string, password?: string) => {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('card_id', cardId);
      if (password) formData.append('password', password);
      const res = await fetch(`${BASE}/statements/upload`, { method: 'POST', body: formData });
      if (!res.ok) throw new ApiError(res.status, await res.text());
      return res.json();
    },
    uploadBulk: async (files: File[], opts: { bank?: string; cardId?: string; password?: string }) => {
      const formData = new FormData();
      if (opts.bank) formData.append('bank', opts.bank);
      if (opts.cardId) formData.append('card_id', opts.cardId);
      if (opts.password) formData.append('password', opts.password);
      for (const file of files) {
        formData.append('files', file);
      }
      const res = await fetch(`${BASE}/statements/upload-bulk`, { method: 'POST', body: formData });
      if (!res.ok) throw new ApiError(res.status, await res.text());
      return res.json();
    },
    delete: (id: string) =>
      request<void>(`/statements/${id}`, { method: 'DELETE' }),
  },

  // Analytics
  analytics: {
    summary: (params: { date?: string; card_id?: string } = {}) =>
      request<SpendSummary>(`/analytics/summary?${qs(params)}`),
    trends: (params: { months?: number; card_id?: string } = {}) =>
      request<MonthTrend[]>(`/analytics/trends?${qs(params)}`),
    calendar: (params: { year?: number; card_id?: string } = {}) =>
      request<Record<string, number>>(`/analytics/calendar?${qs(params)}`),
  },

  // Auth
  auth: {
    accounts: () => request<OAuthAccount[]>('/auth/accounts'),
    deleteAccount: (id: string) =>
      request<void>(`/auth/accounts/${id}`, { method: 'DELETE' }),
    loginUrl: () => '/auth/google/login',
  },

  // Sync
  sync: {
    trigger: () => request<void>('/sync', { method: 'POST' }),
    status: () => request<{ status: string }>('/sync/status'),
    errors: () => request<SyncError[]>('/sync/errors'),
    deleteError: (id: string) => request<void>(`/sync/errors/${id}`, { method: 'DELETE' }),
  },

  // Settings
  settings: {
    get: () => request<Record<string, string>>('/settings'),
    update: (settings: Record<string, string>) =>
      request<Record<string, string>>('/settings', { method: 'PUT', body: JSON.stringify(settings) }),
  },

  // Categories
  categories: {
    list: () => request<Record<string, string[]>>('/categories'),
  },
};

export { ApiError };
