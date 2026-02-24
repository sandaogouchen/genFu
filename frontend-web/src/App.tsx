import { BrowserRouter as Router, Routes, Route, Navigate } from "react-router-dom";
import AppShell from "@/components/AppShell";
import Analyze from "@/pages/Analyze";
import Chat from "@/pages/Chat";
import Decision from "@/pages/Decision";
import Docs from "@/pages/Docs";
import Home from "@/pages/Home";
import Investment from "@/pages/Investment";
import Market from "@/pages/Market";
import News from "@/pages/News";
import ReportDetail from "@/pages/ReportDetail";
import Reports from "@/pages/Reports";
import StockPicker from "@/pages/StockPicker";
import Workflow from "@/pages/Workflow";

export default function App() {
  return (
    <Router>
      <AppShell>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/analyze" element={<Analyze />} />
          <Route path="/reports" element={<Reports />} />
          <Route path="/reports/:id" element={<ReportDetail />} />
          <Route path="/decision" element={<Decision />} />
          <Route path="/chat" element={<Chat />} />
          <Route path="/investment" element={<Investment />} />
          <Route path="/market" element={<Market />} />
          <Route path="/news" element={<News />} />
          <Route path="/stockpicker" element={<StockPicker />} />
          <Route path="/workflow" element={<Workflow />} />
          <Route path="/docs" element={<Docs />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell>
    </Router>
  );
}
