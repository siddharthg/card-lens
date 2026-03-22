import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { OAuthAccount } from '../types';

interface CategoryRule {
  id: string;
  pattern: string;
  match_type: string;
  merchant: string;
  company: string;
  category: string;
  sub_category: string;
  priority: number;
  is_builtin: boolean;
}

interface MerchantRulesResponse {
  builtin: { Pattern: string; Merchant: string; Company: string; Category: string; SubCategory: string }[];
  custom: CategoryRule[];
}

export default function Settings() {
  const queryClient = useQueryClient();
  const [showRuleForm, setShowRuleForm] = useState(false);
  const [ruleForm, setRuleForm] = useState({
    pattern: '',
    merchant: '',
    company: '',
    category: '',
    sub_category: '',
    priority: 100,
  });

  const { data: settings } = useQuery<Record<string, string>>({
    queryKey: ['settings'],
    queryFn: () => api.settings.get(),
  });

  const settingsMutation = useMutation({
    mutationFn: (s: Record<string, string>) => api.settings.update(s),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['settings'] }),
  });

  const { data: accounts } = useQuery<OAuthAccount[]>({
    queryKey: ['oauth-accounts'],
    queryFn: () => api.auth.accounts(),
  });

  const { data: syncStatus } = useQuery<{ status: string; message?: string; last_sync?: string; processed?: number }>({
    queryKey: ['sync-status'],
    queryFn: () => api.sync.status(),
    refetchInterval: (query) => {
      return query.state.data?.status === 'syncing' ? 2000 : false;
    },
  });

  const { data: categories } = useQuery<Record<string, string[]>>({
    queryKey: ['categories'],
    queryFn: () => api.categories.list(),
  });

  const { data: merchantRules } = useQuery<MerchantRulesResponse>({
    queryKey: ['merchant-rules'],
    queryFn: async () => {
      const res = await fetch('/api/merchants/rules');
      return res.json();
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.auth.deleteAccount(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['oauth-accounts'] }),
  });

  const syncMutation = useMutation({
    mutationFn: () => api.sync.trigger(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sync-status'] });
    },
  });

  const createRuleMutation = useMutation({
    mutationFn: async (rule: typeof ruleForm) => {
      const res = await fetch('/api/merchants/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...rule, match_type: 'contains' }),
      });
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['merchant-rules'] });
      setShowRuleForm(false);
      setRuleForm({ pattern: '', merchant: '', company: '', category: '', sub_category: '', priority: 100 });
    },
  });

  const deleteRuleMutation = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/merchants/rules/${id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error(await res.text());
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['merchant-rules'] }),
  });

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      {/* Gmail Connection */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-4">Gmail Connection</h2>
        <p className="text-sm text-gray-400 mb-4">
          Connect your Gmail to automatically fetch credit card statement PDFs.
        </p>

        {(accounts?.length ?? 0) > 0 && (
          <div className="mb-4 space-y-2">
            {accounts!.map((a) => (
              <div key={a.id} className="flex items-center justify-between bg-gray-800 rounded-lg px-4 py-3">
                <div>
                  <p className="font-medium">{a.email}</p>
                  <p className="text-xs text-gray-500">Connected {new Date(a.created_at).toLocaleDateString()}</p>
                </div>
                <button
                  className="text-xs text-red-400 hover:text-red-300"
                  onClick={() => { if (confirm('Disconnect this account?')) deleteMutation.mutate(a.id); }}
                >
                  Disconnect
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="flex gap-3">
          <a href={api.auth.loginUrl()} className="btn-primary inline-block">
            {(accounts?.length ?? 0) > 0 ? 'Add Another Gmail Account' : 'Connect Gmail'}
          </a>
          {(accounts?.length ?? 0) > 0 && (
            <button
              className="btn-secondary"
              onClick={() => syncMutation.mutate()}
              disabled={syncMutation.isPending || syncStatus?.status === 'syncing'}
            >
              {syncStatus?.status === 'syncing' ? 'Syncing...' : 'Sync Now'}
            </button>
          )}
        </div>
        {syncStatus && syncStatus.status !== 'idle' && (
          <div className="mt-3 text-sm">
            <span className={`inline-block px-2 py-0.5 rounded text-xs ${
              syncStatus.status === 'syncing' ? 'bg-blue-900 text-blue-300' :
              syncStatus.status === 'completed' ? 'bg-green-900 text-green-300' :
              syncStatus.status === 'error' ? 'bg-red-900 text-red-300' :
              'bg-gray-800 text-gray-400'
            }`}>
              {syncStatus.status}
            </span>
            {syncStatus.message && <span className="text-gray-400 ml-2">{syncStatus.message}</span>}
            {syncStatus.last_sync && (
              <p className="text-xs text-gray-500 mt-1">
                Last sync: {new Date(syncStatus.last_sync).toLocaleString()}
              </p>
            )}
          </div>
        )}
      </div>

      {/* Global Profile & Decryption */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-4">Profile & Statement Decryption</h2>
        <p className="text-sm text-gray-400 mb-4">
          Your name, DOB, and PAN are used to decrypt bank statement PDFs and auto-create cards during sync.
        </p>
        <form
          className="grid grid-cols-1 sm:grid-cols-3 gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            const form = e.target as HTMLFormElement;
            const card_holder = (form.elements.namedItem('card_holder') as HTMLInputElement).value;
            const dob = (form.elements.namedItem('dob') as HTMLInputElement).value;
            const pan = (form.elements.namedItem('pan') as HTMLInputElement).value;
            settingsMutation.mutate({ card_holder, dob, pan });
          }}
        >
          <div>
            <label className="block text-xs text-gray-400 mb-1">Card Holder Name</label>
            <input
              name="card_holder"
              className="input w-full text-sm"
              placeholder="e.g. SIDDHARTH GUPTA"
              defaultValue={settings?.card_holder ?? ''}
              key={settings?.card_holder}
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Date of Birth (DDMMYYYY)</label>
            <input
              name="dob"
              className="input w-full text-sm"
              placeholder="e.g. 15061990"
              defaultValue={settings?.dob ?? ''}
              key={settings?.dob}
              maxLength={8}
              pattern="\d{8}"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">PAN</label>
            <input
              name="pan"
              className="input w-full text-sm"
              placeholder="e.g. ABCDE1234F"
              defaultValue={settings?.pan ?? ''}
              key={settings?.pan}
              maxLength={10}
            />
          </div>
          <div className="sm:col-span-3">
            <button type="submit" className="btn-primary text-sm" disabled={settingsMutation.isPending}>
              {settingsMutation.isPending ? 'Saving...' : 'Save'}
            </button>
            {settingsMutation.isSuccess && (
              <span className="text-green-400 text-sm ml-3">Saved</span>
            )}
          </div>
        </form>
      </div>

      {/* Custom Merchant Rules */}
      <div className="card mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Custom Merchant Rules</h2>
          <button className="btn-secondary text-sm" onClick={() => setShowRuleForm(!showRuleForm)}>
            {showRuleForm ? 'Cancel' : 'Add Rule'}
          </button>
        </div>
        <p className="text-sm text-gray-400 mb-4">
          Custom rules take priority over built-in rules. Match transactions by description pattern.
        </p>

        {showRuleForm && (
          <form
            className="bg-gray-800 rounded-lg p-4 mb-4 grid grid-cols-1 sm:grid-cols-2 gap-3"
            onSubmit={(e) => { e.preventDefault(); createRuleMutation.mutate(ruleForm); }}
          >
            <div>
              <label className="block text-xs text-gray-400 mb-1">Pattern (matched in description)</label>
              <input className="input w-full text-sm" placeholder="e.g. STARBUCKS" value={ruleForm.pattern} onChange={(e) => setRuleForm({ ...ruleForm, pattern: e.target.value })} required />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Category</label>
              <select className="select w-full text-sm" value={ruleForm.category} onChange={(e) => setRuleForm({ ...ruleForm, category: e.target.value })} required>
                <option value="">Select...</option>
                {Object.keys(categories ?? {}).map((c) => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Merchant Name</label>
              <input className="input w-full text-sm" placeholder="e.g. Starbucks" value={ruleForm.merchant} onChange={(e) => setRuleForm({ ...ruleForm, merchant: e.target.value })} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Sub Category</label>
              <input className="input w-full text-sm" placeholder="e.g. Cafe" value={ruleForm.sub_category} onChange={(e) => setRuleForm({ ...ruleForm, sub_category: e.target.value })} />
            </div>
            <div className="sm:col-span-2">
              <button type="submit" className="btn-primary text-sm" disabled={createRuleMutation.isPending}>
                {createRuleMutation.isPending ? 'Creating...' : 'Create Rule'}
              </button>
            </div>
          </form>
        )}

        {/* Custom rules list */}
        {(merchantRules?.custom?.length ?? 0) > 0 && (
          <div className="space-y-2 mb-4">
            <h3 className="text-sm font-medium text-gray-300">Your Rules</h3>
            {merchantRules!.custom.map((rule) => (
              <div key={rule.id} className="flex items-center justify-between bg-gray-800 rounded-lg px-3 py-2">
                <div className="text-sm">
                  <span className="font-mono text-blue-400">{rule.pattern}</span>
                  <span className="text-gray-500 mx-2">→</span>
                  <span>{rule.merchant || rule.category}</span>
                  <span className="text-gray-500 ml-2">({rule.category})</span>
                </div>
                <button
                  className="text-xs text-red-400 hover:text-red-300"
                  onClick={() => deleteRuleMutation.mutate(rule.id)}
                >
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Builtin rules count */}
        <p className="text-xs text-gray-500">
          {merchantRules?.builtin?.length ?? 0} built-in rules active
        </p>
      </div>

      {/* Categories */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-4">Categories</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {Object.entries(categories ?? {}).map(([cat, subs]) => (
            <div key={cat} className="bg-gray-800 rounded-lg px-3 py-2">
              <p className="font-medium text-sm">{cat}</p>
              {subs.length > 0 && (
                <p className="text-xs text-gray-500 mt-1">{subs.join(', ')}</p>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Export */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-4">Export Data</h2>
        <p className="text-sm text-gray-400 mb-4">Download all transactions as CSV.</p>
        <a href={api.transactions.export({})} className="btn-secondary inline-block" download>
          Export All Transactions
        </a>
      </div>

      {/* About */}
      <div className="card">
        <h2 className="text-lg font-semibold mb-2">About CardLens</h2>
        <p className="text-sm text-gray-400">
          Local-first credit card expense tracker. Your data stays on your machine.
        </p>
        <p className="text-xs text-gray-600 mt-2">All data stored locally in SQLite.</p>
      </div>
    </div>
  );
}
