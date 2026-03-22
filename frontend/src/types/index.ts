export interface CreditCard {
  id: string;
  bank: string;
  card_name: string;
  last_four: string;
  billing_day: number;
  card_holder: string;
  addon_holders: string[];
  stmt_password?: string;
  created_at: string;
  updated_at: string;
}

export interface Transaction {
  id: string;
  card_id: string;
  statement_id?: string;
  txn_date: string;
  post_date?: string;
  description: string;
  amount: number;
  currency: string;
  is_international: boolean;
  merchant: string;
  company: string;
  category: string;
  sub_category: string;
  spender: string;
  is_recurring: boolean;
  tags: string[];
  notes: string;
  created_at: string;
}

export interface Statement {
  id: string;
  card_id: string;
  gmail_msg_id?: string;
  filename: string;
  period_start?: string;
  period_end?: string;
  total_amount: number;
  prev_balance: number;
  purchase_total: number;
  payments_total: number;
  minimum_due: number;
  due_date?: string;
  status: string;
  validation_message: string;
  txn_count: number;
  parsed_at: string;
}

export interface TransactionListResult {
  transactions: Transaction[];
  total: number;
  page: number;
  limit: number;
}

export interface SpendSummary {
  period: string;
  total_spend: number;
  by_category: Record<string, number>;
  by_spender: Record<string, number>;
  by_card: Record<string, number>;
  top_merchants: MerchantSpend[];
  daily_spend: Record<string, number>;
}

export interface MerchantSpend {
  merchant: string;
  category: string;
  amount: number;
  count: number;
}

export interface OAuthAccount {
  id: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface MonthTrend {
  month: string;
  total: number;
}

export interface SyncError {
  id: string;
  gmail_msg_id: string;
  bank: string;
  filename: string;
  email_subject: string;
  error: string;
  created_at: string;
}

// Bank color mapping
export const BANK_COLORS: Record<string, string> = {
  HDFC: '#004B87',
  ICICI: '#F58220',
  SBI: '#22409A',
  Amex: '#006FCF',
  Axis: '#97144D',
  'IDFC First': '#9C1D26',
  IndusInd: '#7B2D8E',
  HSBC: '#DB0011',
};

// Format amount in Indian numbering system
export function formatINR(amount: number): string {
  const isNegative = amount < 0;
  const abs = Math.abs(amount);
  const [whole, decimal] = abs.toFixed(2).split('.');

  // Indian numbering: last 3 digits, then groups of 2
  let result = '';
  const len = whole.length;
  if (len <= 3) {
    result = whole;
  } else {
    result = whole.slice(-3);
    let remaining = whole.slice(0, -3);
    while (remaining.length > 2) {
      result = remaining.slice(-2) + ',' + result;
      remaining = remaining.slice(0, -2);
    }
    if (remaining.length > 0) {
      result = remaining + ',' + result;
    }
  }

  return (isNegative ? '-' : '') + '\u20B9' + result + '.' + decimal;
}
