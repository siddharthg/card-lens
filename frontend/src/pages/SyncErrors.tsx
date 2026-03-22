import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { SyncError } from '../types';
import { BANK_COLORS } from '../types';

export default function SyncErrors() {
  const queryClient = useQueryClient();

  const { data: errors, isLoading } = useQuery<SyncError[]>({
    queryKey: ['sync-errors'],
    queryFn: () => api.sync.errors(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.sync.deleteError(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['sync-errors'] }),
  });

  // Group errors by type
  const passwordErrors = errors?.filter((e) => e.error.includes('password')) ?? [];
  const parseErrors = errors?.filter((e) => !e.error.includes('password') && !e.error.includes('no matching card') && !e.error.includes('no card number')) ?? [];
  const cardErrors = errors?.filter((e) => e.error.includes('no matching card') || e.error.includes('no card number')) ?? [];

  const bankGroups = new Map<string, SyncError[]>();
  for (const e of errors ?? []) {
    const bank = e.bank || 'Unknown';
    if (!bankGroups.has(bank)) bankGroups.set(bank, []);
    bankGroups.get(bank)!.push(e);
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Sync Errors</h1>
        <p className="text-sm text-gray-400">{errors?.length ?? 0} unresolved</p>
      </div>

      {isLoading ? (
        <p className="text-gray-500">Loading...</p>
      ) : !errors?.length ? (
        <div className="card text-center py-12">
          <p className="text-gray-400 text-lg mb-2">No sync errors</p>
          <p className="text-gray-500 text-sm">All emails were parsed successfully</p>
        </div>
      ) : (
        <>
          {/* Summary cards */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
            <div className="card">
              <p className="text-sm text-red-400">Password Errors</p>
              <p className="text-2xl font-bold text-red-400">{passwordErrors.length}</p>
              <p className="text-xs text-gray-500 mt-1">Check DOB/PAN in Settings or set card password</p>
            </div>
            <div className="card">
              <p className="text-sm text-yellow-400">Parse Errors</p>
              <p className="text-2xl font-bold text-yellow-400">{parseErrors.length}</p>
              <p className="text-xs text-gray-500 mt-1">PDF could not be parsed (format issue)</p>
            </div>
            <div className="card">
              <p className="text-sm text-blue-400">Card Matching</p>
              <p className="text-2xl font-bold text-blue-400">{cardErrors.length}</p>
              <p className="text-xs text-gray-500 mt-1">Parsed but no card to assign to</p>
            </div>
          </div>

          {/* Errors by bank */}
          {[...bankGroups.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([bank, bankErrors]) => {
            const color = BANK_COLORS[bank] ?? '#6B7280';
            return (
              <div key={bank} className="mb-6">
                <h2 className="text-lg font-semibold mb-3" style={{ color }}>
                  {bank} <span className="text-gray-500 text-sm font-normal">({bankErrors.length})</span>
                </h2>
                <div className="space-y-2">
                  {bankErrors.map((e) => (
                    <div key={e.id} className="card border border-gray-800 py-3 px-4">
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1">
                            <ErrorBadge error={e.error} />
                            <span className="text-sm font-mono truncate">{e.filename}</span>
                          </div>
                          {e.email_subject && (
                            <p className="text-xs text-gray-500 truncate mb-1">{e.email_subject}</p>
                          )}
                          <p className="text-xs text-red-400">{e.error}</p>
                        </div>
                        <div className="flex items-center gap-3 shrink-0">
                          <span className="text-xs text-gray-600">
                            {new Date(e.created_at).toLocaleDateString()}
                          </span>
                          <button
                            className="text-xs text-gray-500 hover:text-red-400"
                            onClick={() => deleteMutation.mutate(e.id)}
                            title="Dismiss"
                          >
                            Dismiss
                          </button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </>
      )}
    </div>
  );
}

function ErrorBadge({ error }: { error: string }) {
  let label = 'error';
  let style = 'bg-gray-800 text-gray-400 border-gray-700';

  if (error.includes('password')) {
    label = 'password';
    style = 'bg-red-900/30 text-red-400 border-red-800/50';
  } else if (error.includes('no matching card') || error.includes('no card number')) {
    label = 'no card';
    style = 'bg-blue-900/30 text-blue-400 border-blue-800/50';
  } else if (error.includes('no parser') || error.includes('no text')) {
    label = 'format';
    style = 'bg-yellow-900/30 text-yellow-400 border-yellow-800/50';
  } else {
    label = 'parse';
    style = 'bg-yellow-900/30 text-yellow-400 border-yellow-800/50';
  }

  return (
    <span className={`text-xs px-2 py-0.5 rounded border ${style}`}>
      {label}
    </span>
  );
}
