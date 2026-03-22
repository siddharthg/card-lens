import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Transactions from './pages/Transactions';
import Statements from './pages/Statements';
import StatementDetail from './pages/StatementDetail';
import Cards from './pages/Cards';
import Insights from './pages/Insights';
import Settings from './pages/Settings';
import SyncErrors from './pages/SyncErrors';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/transactions" element={<Transactions />} />
            <Route path="/statements" element={<Statements />} />
            <Route path="/statements/:id" element={<StatementDetail />} />
            <Route path="/cards" element={<Cards />} />
            <Route path="/insights" element={<Insights />} />
            <Route path="/sync-errors" element={<SyncErrors />} />
            <Route path="/settings" element={<Settings />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
