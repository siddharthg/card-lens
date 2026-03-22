import { useState } from 'react';
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import type { CreditCard } from '../types';

interface Props {
  cardId?: string;
  onSuccess?: () => void;
}

export default function StatementUpload({ cardId, onSuccess }: Props) {
  const queryClient = useQueryClient();
  const [files, setFiles] = useState<File[]>([]);
  const [password, setPassword] = useState('');
  const [bank, setBank] = useState('');
  const [resultSummary, setResultSummary] = useState<string | null>(null);

  const { data: cards } = useQuery<CreditCard[]>({
    queryKey: ['cards'],
    queryFn: () => api.cards.list(),
    enabled: !cardId,
  });

  const banks = [...new Set(cards?.map((c) => c.bank) ?? [])].sort();

  const isBulk = files.length > 1;

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (files.length === 0) throw new Error('No file selected');
      if (cardId && !isBulk) {
        return api.statements.upload(files[0], cardId, password || undefined);
      }
      return api.statements.uploadBulk(files, {
        bank: bank || undefined,
        cardId: cardId || undefined,
        password: password || undefined,
      });
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['statements'] });
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
      queryClient.invalidateQueries({ queryKey: ['summary'] });
      queryClient.invalidateQueries({ queryKey: ['cards'] });
      setFiles([]);
      setPassword('');
      onSuccess?.();

      if (data.results) {
        const results = data.results as any[];
        const parsed = results.filter((r) => r.status === 'parsed' || r.status === 'failed');
        const dupes = results.filter((r) => r.status === 'duplicate');
        const errors = results.filter((r) => r.status === 'error');
        const empty = results.filter((r) => r.status === 'empty');
        const totalTxns = data.total_transactions ?? 0;

        const parts: string[] = [];
        if (parsed.length > 0) parts.push(`${parsed.length} parsed (${totalTxns} txns)`);
        if (dupes.length > 0) parts.push(`${dupes.length} duplicates`);
        if (empty.length > 0) parts.push(`${empty.length} empty`);
        if (errors.length > 0) parts.push(`${errors.length} errors`);

        setResultSummary(`${data.total_files} files: ${parts.join(', ')}`);

        if (errors.length > 0) {
          const errorDetails = errors.map((r: any) => `${r.filename}: ${r.error}`).join(', ');
          setResultSummary((prev) => prev + ` | Errors: ${errorDetails}`);
        }
      } else {
        setResultSummary(`${data.transactions} transactions imported`);
      }
    },
  });

  return (
    <div className="space-y-3">
      {/* Bank selector (only when no explicit cardId) */}
      {!cardId && (
        <div>
          <label className="block text-sm text-gray-400 mb-1">Bank (auto-detects card from statement)</label>
          <select
            className="select w-full"
            value={bank}
            onChange={(e) => setBank(e.target.value)}
          >
            <option value="">All banks (auto-detect)</option>
            {banks.map((b) => (
              <option key={b} value={b}>{b}</option>
            ))}
          </select>
        </div>
      )}

      <div>
        <label className="block text-sm text-gray-400 mb-1">Statement PDF(s)</label>
        <input
          type="file"
          accept=".pdf"
          multiple
          className="input w-full text-sm file:mr-3 file:py-1 file:px-3 file:rounded file:border-0 file:bg-gray-700 file:text-gray-200 file:cursor-pointer"
          onChange={(e) => {
            setFiles(Array.from(e.target.files ?? []));
            setResultSummary(null);
          }}
        />
        {files.length > 1 && (
          <p className="text-xs text-gray-500 mt-1">{files.length} files selected</p>
        )}
      </div>

      <div>
        <label className="block text-sm text-gray-400 mb-1">PDF Password (optional)</label>
        <input
          type="password"
          className="input w-full"
          placeholder="Leave blank if not password protected"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </div>

      <button
        className="btn-primary w-full"
        disabled={files.length === 0 || uploadMutation.isPending}
        onClick={() => {
          setResultSummary(null);
          uploadMutation.mutate();
        }}
      >
        {uploadMutation.isPending
          ? `Uploading ${files.length} file${files.length > 1 ? 's' : ''}...`
          : `Upload & Parse${files.length > 1 ? ` ${files.length} Statements` : ' Statement'}`}
      </button>

      {resultSummary && (
        <p className="text-green-400 text-sm">{resultSummary}</p>
      )}

      {uploadMutation.isError && (
        <p className="text-red-400 text-sm">{(uploadMutation.error as Error).message}</p>
      )}
    </div>
  );
}
