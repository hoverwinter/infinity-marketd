import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Database,
  Gauge,
  Network,
  Play,
  RefreshCw,
  Search,
  Server,
  TerminalSquare
} from "lucide-react";
import {
  api,
  BestIPStatus,
  ConsoleSummary,
  DataQualityIssue,
  HQDailyImportSummary,
  ProbeResult,
  Quote,
  QuoteServiceRun,
  TaskRun,
  Watermark
} from "./api";

type ViewKey = "overview" | "ops" | "realtime" | "bestip" | "smoke";

type Loadable<T> = {
  loading: boolean;
  error: string;
  data: T | null;
};

const tabs: Array<{ key: ViewKey; label: string; icon: typeof Activity }> = [
  { key: "overview", label: "Overview", icon: Gauge },
  { key: "ops", label: "Ops", icon: Database },
  { key: "realtime", label: "Realtime", icon: Activity },
  { key: "bestip", label: "BestIP", icon: Network },
  { key: "smoke", label: "TDX Smoke", icon: TerminalSquare }
];

const initialLoadable = <T,>(): Loadable<T> => ({ loading: true, error: "", data: null });

export default function App() {
  const [active, setActive] = useState<ViewKey>("overview");
  const [summary, setSummary] = useState<Loadable<ConsoleSummary>>(initialLoadable);
  const [watermarks, setWatermarks] = useState<Loadable<Watermark[]>>(initialLoadable);
  const [taskRuns, setTaskRuns] = useState<Loadable<TaskRun[]>>(initialLoadable);
  const [issues, setIssues] = useState<Loadable<DataQualityIssue[]>>(initialLoadable);
  const [quoteRuns, setQuoteRuns] = useState<Loadable<QuoteServiceRun[]>>(initialLoadable);
  const [bestip, setBestip] = useState<Loadable<BestIPStatus>>(initialLoadable);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);

  const refreshAll = async () => {
    await Promise.all([
      loadState(setSummary, () => api.summary(20)),
      loadState(setWatermarks, () => api.watermarks(50)),
      loadState(setTaskRuns, () => api.taskRuns(50)),
      loadState(setIssues, () => api.dataQualityIssues(50)),
      loadState(setQuoteRuns, () => api.quoteRuns(50)),
      loadState(setBestip, () => api.bestip())
    ]);
    setLastRefresh(new Date());
  };

  useEffect(() => {
    void refreshAll();
  }, []);

  const staleText = lastRefresh ? `Updated ${formatClock(lastRefresh.toISOString())}` : "Not loaded";

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">∞</div>
          <div>
            <div className="brand-name">Infinity Console</div>
            <div className="brand-subtitle">marketd operations</div>
          </div>
        </div>
        <nav className="nav-list" aria-label="Console sections">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                className={`nav-item ${active === tab.key ? "active" : ""}`}
                key={tab.key}
                onClick={() => setActive(tab.key)}
                type="button"
              >
                <Icon size={18} />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>
        <div className="sidebar-footer">
          <span className="status-dot" />
          <span>{summary.data?.health.status ?? "unknown"}</span>
        </div>
      </aside>
      <main className="workspace">
        <header className="topbar">
          <div>
            <h1>{tabs.find((tab) => tab.key === active)?.label}</h1>
            <p>{subtitleFor(active)}</p>
          </div>
          <button className="icon-button" type="button" onClick={() => void refreshAll()} title="Refresh all console data">
            <RefreshCw size={18} />
            <span>Refresh</span>
          </button>
        </header>
        <div className="freshness">{staleText}</div>
        {active === "overview" && <Overview state={summary} />}
        {active === "ops" && <Ops watermarks={watermarks} taskRuns={taskRuns} issues={issues} onAfterImport={refreshAll} />}
        {active === "realtime" && <Realtime quoteRuns={quoteRuns} summary={summary} />}
        {active === "bestip" && <BestIP state={bestip} reload={() => loadState(setBestip, () => api.bestip())} />}
        {active === "smoke" && <Smoke />}
      </main>
    </div>
  );
}

function Overview({ state }: { state: Loadable<ConsoleSummary> }) {
  if (state.loading) return <Loading label="Loading overview" />;
  if (state.error) return <ErrorState message={state.error} />;
  if (!state.data) return <EmptyState label="No overview data" />;

  const latestQuoteRun = state.data.quote_service_runs[0];
  return (
    <section className="view-stack">
      <div className="metrics-grid">
        <Metric label="Health" value={state.data.health.status} sub={`schema ${state.data.health.schema_version}`} icon={Server} />
        <Metric label="Watermarks" value={state.data.watermarks.length.toString()} sub="recent assets" icon={Database} />
        <Metric label="Task runs" value={state.data.task_runs.length.toString()} sub="latest imports" icon={BarChart3} />
        <Metric
          label="Quote rows"
          value={latestQuoteRun ? latestQuoteRun.rows_fetched.toLocaleString() : "0"}
          sub={latestQuoteRun?.status ?? "no recent run"}
          icon={Activity}
        />
      </div>
      <Split>
        <Panel title="Recent watermarks" icon={Database}>
          <WatermarkTable rows={state.data.watermarks.slice(0, 8)} />
        </Panel>
        <Panel title="Quality issue counts" icon={AlertTriangle}>
          <SimpleTable
            empty="No recent quality issues"
            columns={["Dataset", "Severity", "Type", "Count"]}
            rows={state.data.data_quality_issue_counts.map((item) => [
              item.dataset,
              item.severity,
              item.issue_type,
              item.count.toLocaleString()
            ])}
          />
        </Panel>
      </Split>
      <Panel title="Latest task runs" icon={TerminalSquare}>
        <TaskRunTable rows={state.data.task_runs.slice(0, 8)} />
      </Panel>
    </section>
  );
}

function Ops({
  watermarks,
  taskRuns,
  issues,
  onAfterImport
}: {
  watermarks: Loadable<Watermark[]>;
  taskRuns: Loadable<TaskRun[]>;
  issues: Loadable<DataQualityIssue[]>;
  onAfterImport: () => Promise<void>;
}) {
  return (
    <section className="view-stack">
      <Panel title="Online daily import" icon={Play}>
        <HQDailyImportForm onAfterImport={onAfterImport} />
      </Panel>
      <Panel title="Watermarks" icon={Database}>
        <LoadableTable state={watermarks} render={(rows) => <WatermarkTable rows={rows} />} />
      </Panel>
      <Panel title="Task runs" icon={TerminalSquare}>
        <LoadableTable state={taskRuns} render={(rows) => <TaskRunTable rows={rows} />} />
      </Panel>
      <Panel title="Data quality issues" icon={AlertTriangle}>
        <LoadableTable state={issues} render={(rows) => <IssueTable rows={rows} />} />
      </Panel>
    </section>
  );
}

function HQDailyImportForm({ onAfterImport }: { onAfterImport: () => Promise<void> }) {
  const [market, setMarket] = useState("sh");
  const [symbol, setSymbol] = useState("600519");
  const [since, setSince] = useState("");
  const [until, setUntil] = useState("");
  const [start, setStart] = useState("0");
  const [count, setCount] = useState("800");
  const [servers, setServers] = useState("");
  const [dryRun, setDryRun] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<HQDailyImportSummary | null>(null);

  const submit = async () => {
    setSubmitting(true);
    setError("");
    try {
      const summary = await api.importHQDaily({
        market,
        symbol,
        since,
        until,
        start: Number(start || 0),
        count: Number(count || 0),
        servers,
        dryRun
      });
      setResult(summary);
      if (!summary.dry_run) await onAfterImport();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Import failed");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="online-import">
      <div className="import-form">
        <label className="import-field">
          <span>Market</span>
          <div className="market-toggle">
            {["sh", "sz", "bj"].map((item) => (
              <button className={market === item ? "active" : ""} key={item} type="button" onClick={() => setMarket(item)}>
                {item}
              </button>
            ))}
          </div>
        </label>
        <label className="import-field">
          <span>Symbol</span>
          <input value={symbol} onChange={(event) => setSymbol(event.target.value)} />
        </label>
        <label className="import-field">
          <span>Since</span>
          <input type="date" value={since} onChange={(event) => setSince(event.target.value)} />
        </label>
        <label className="import-field">
          <span>Until</span>
          <input type="date" value={until} onChange={(event) => setUntil(event.target.value)} />
        </label>
        <label className="import-field">
          <span>Start</span>
          <input type="number" min="0" value={start} onChange={(event) => setStart(event.target.value)} />
        </label>
        <label className="import-field">
          <span>Count</span>
          <input type="number" min="1" max="51200" value={count} onChange={(event) => setCount(event.target.value)} />
        </label>
        <label className="import-field server-field">
          <span>Servers</span>
          <input value={servers} onChange={(event) => setServers(event.target.value)} placeholder="host:7709,host:7709" />
        </label>
        <label className="dry-run-toggle">
          <input type="checkbox" checked={dryRun} onChange={(event) => setDryRun(event.target.checked)} />
          <span>Dry run</span>
        </label>
        <button className="primary-button" type="button" onClick={() => void submit()} disabled={submitting}>
          <Play size={17} />
          <span>{submitting ? "Running" : dryRun ? "Preview" : "Import"}</span>
        </button>
      </div>
      {error && <p className="inline-error">{error}</p>}
      {result && (
        <div className="metrics-grid compact import-result">
          <Metric label={result.dry_run ? "Would write" : "Rows written"} value={result.rows_written.toLocaleString()} sub={`${result.rows_fetched.toLocaleString()} fetched`} icon={Database} />
          <Metric label="Pages" value={result.pages_fetched.toString()} sub={`${result.rows_skipped.toLocaleString()} skipped`} icon={BarChart3} />
          <Metric label="Issues" value={result.issues.length.toString()} sub={result.dry_run ? "dry run" : shortID(result.run_id)} icon={AlertTriangle} />
        </div>
      )}
    </div>
  );
}

function Realtime({ quoteRuns, summary }: { quoteRuns: Loadable<QuoteServiceRun[]>; summary: Loadable<ConsoleSummary> }) {
  const latest = summary.data?.quote_service_runs[0];
  return (
    <section className="view-stack">
      <div className="metrics-grid compact">
        <Metric label="Latest run" value={latest?.status ?? "none"} sub={latest?.run_id ?? "no run"} icon={Activity} />
        <Metric label="Planned batches" value={(latest?.planned_batches ?? 0).toLocaleString()} sub="last sweep" icon={BarChart3} />
        <Metric label="Failures" value={(latest?.failed_batches ?? 0).toLocaleString()} sub="last sweep" icon={AlertTriangle} />
      </div>
      <Panel title="Quote service runs" icon={Activity}>
        <LoadableTable state={quoteRuns} render={(rows) => <QuoteRunTable rows={rows} />} />
      </Panel>
    </section>
  );
}

function BestIP({ state, reload }: { state: Loadable<BestIPStatus>; reload: () => Promise<void> }) {
  const [servers, setServers] = useState("");
  const [maxAge, setMaxAge] = useState("24h");
  const [refreshing, setRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState("");
  const results = state.data?.results ?? [];
  const hasCache = Boolean(state.data?.generated_at || state.data?.expires_at || results.length > 0);

  const refresh = async () => {
    setRefreshing(true);
    setRefreshError("");
    try {
      await api.refreshBestip(servers, maxAge);
      await reload();
    } catch (error) {
      setRefreshError(error instanceof Error ? error.message : "Refresh failed");
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <section className="view-stack">
      {state.loading && <Loading label="Loading bestip cache" />}
      {state.error && <ErrorState message={state.error} />}
      {state.data && (
        <>
          <div className="metrics-grid compact">
            <Metric label="Preferred" value={state.data.preferred || "none"} sub={state.data.usable ? "cache usable" : "cache unavailable"} icon={Network} />
            <Metric label="Results" value={results.length.toString()} sub={state.data.cache_path} icon={Server} />
            <Metric label="Expires" value={formatClock(state.data.expires_at)} sub="bestip cache" icon={Gauge} />
          </div>
          {state.data.error && <ErrorState message={state.data.error} />}
          <Panel title={hasCache ? "Refresh bestip" : "Generate bestip"} icon={RefreshCw}>
            <div className="form-row">
              <label>
                <span>Servers</span>
                <input value={servers} onChange={(event) => setServers(event.target.value)} placeholder="host:7709,host:7709" />
              </label>
              <label>
                <span>Max age</span>
                <input value={maxAge} onChange={(event) => setMaxAge(event.target.value)} />
              </label>
              <button className="primary-button" type="button" onClick={() => void refresh()} disabled={refreshing}>
                <RefreshCw size={17} />
                <span>{refreshing ? (hasCache ? "Refreshing" : "Generating") : hasCache ? "Refresh" : "Generate"}</span>
              </button>
            </div>
            {refreshError && <p className="inline-error">{refreshError}</p>}
          </Panel>
          <Panel title="Probe results" icon={Network}>
            <ProbeTable rows={results} />
          </Panel>
        </>
      )}
    </section>
  );
}

function Smoke() {
  const [servers, setServers] = useState("");
  const [symbols, setSymbols] = useState("sh:600519");
  const [probe, setProbe] = useState<Loadable<ProbeResult[]>>({ loading: false, error: "", data: null });
  const [quotes, setQuotes] = useState<Loadable<Quote[]>>({ loading: false, error: "", data: null });

  const runProbe = async () => {
    await loadState(setProbe, async () => (await api.probe(servers)).results);
  };
  const runQuote = async () => {
    await loadState(setQuotes, async () => (await api.quoteSmoke(symbols)).quotes);
  };

  return (
    <section className="view-stack">
      <Panel title="HQ probe" icon={Search}>
        <div className="form-row">
          <label>
            <span>Servers</span>
            <input value={servers} onChange={(event) => setServers(event.target.value)} placeholder="optional host:7709 list" />
          </label>
          <button className="primary-button" type="button" onClick={() => void runProbe()} disabled={probe.loading}>
            <Search size={17} />
            <span>{probe.loading ? "Probing" : "Probe"}</span>
          </button>
        </div>
        {probe.error && <ErrorState message={probe.error} />}
        {probe.data && <ProbeTable rows={probe.data} />}
      </Panel>
      <Panel title="Quote smoke check" icon={Activity}>
        <div className="form-row">
          <label>
            <span>Symbols</span>
            <input value={symbols} onChange={(event) => setSymbols(event.target.value)} placeholder="sh:600519,000001" />
          </label>
          <button className="primary-button" type="button" onClick={() => void runQuote()} disabled={quotes.loading}>
            <Activity size={17} />
            <span>{quotes.loading ? "Fetching" : "Fetch"}</span>
          </button>
        </div>
        {quotes.error && <ErrorState message={quotes.error} />}
        {quotes.data && <QuoteTable rows={quotes.data} />}
      </Panel>
    </section>
  );
}

function WatermarkTable({ rows }: { rows: Watermark[] }) {
  return (
    <SimpleTable
      empty="No watermarks"
      columns={["Dataset", "Asset", "Status", "Rows", "Updated", "Message"]}
      rows={rows.map((row) => [row.dataset, row.asset, statusPill(row.status), row.rows_written.toLocaleString(), formatClock(row.updated_at), row.message])}
    />
  );
}

function TaskRunTable({ rows }: { rows: TaskRun[] }) {
  return (
    <SimpleTable
      empty="No task runs"
      columns={["Run", "Dataset", "Task", "Status", "Rows", "Started", "Error"]}
      rows={rows.map((row) => [
        shortID(row.run_id),
        row.dataset,
        row.task_type,
        statusPill(row.status),
        row.rows_written.toLocaleString(),
        formatClock(row.started_at),
        row.error || "-"
      ])}
    />
  );
}

function IssueTable({ rows }: { rows: DataQualityIssue[] }) {
  return (
    <SimpleTable
      empty="No data quality issues"
      columns={["Observed", "Dataset", "Severity", "Type", "Symbol", "Message"]}
      rows={rows.map((row) => [
        formatClock(row.observed_at),
        row.dataset,
        statusPill(row.severity),
        row.issue_type,
        [row.market, row.symbol].filter(Boolean).join(":") || "-",
        row.message
      ])}
    />
  );
}

function QuoteRunTable({ rows }: { rows: QuoteServiceRun[] }) {
  return (
    <SimpleTable
      empty="No quote service runs"
      columns={["Run", "Status", "Markets", "Batches", "Rows", "Started", "Error"]}
      rows={rows.map((row) => [
        shortID(row.run_id),
        statusPill(row.status),
        (row.markets ?? []).join(",") || "-",
        `${row.succeeded_batches}/${row.planned_batches}`,
        row.rows_fetched.toLocaleString(),
        formatClock(row.started_at),
        row.error || "-"
      ])}
    />
  );
}

function ProbeTable({ rows }: { rows: ProbeResult[] }) {
  return (
    <SimpleTable
      empty="No probe results"
      columns={["Server", "State", "Latency", "Preferred", "Error"]}
      rows={rows.map((row) => [
        row.server,
        statusPill(row.success ? "ok" : "failed"),
        `${row.latency_ms} ms`,
        row.preferred ? "yes" : "-",
        row.error || "-"
      ])}
    />
  );
}

function QuoteTable({ rows }: { rows: Quote[] }) {
  return (
    <SimpleTable
      empty="No quotes"
      columns={["Symbol", "Price", "Open", "High", "Low", "Volume", "Server time"]}
      rows={rows.map((row) => [
        `${row.market}:${row.symbol}`,
        formatNumber(row.price),
        formatNumber(row.open),
        formatNumber(row.high),
        formatNumber(row.low),
        row.volume.toLocaleString(),
        row.quote_time || row.server_intraday_time
      ])}
    />
  );
}

function Panel({ title, icon: Icon, children }: { title: string; icon: typeof Activity; children: ReactNode }) {
  return (
    <section className="panel">
      <div className="panel-title">
        <Icon size={18} />
        <h2>{title}</h2>
      </div>
      {children}
    </section>
  );
}

function Split({ children }: { children: ReactNode }) {
  return <div className="split">{children}</div>;
}

function Metric({ label, value, sub, icon: Icon }: { label: string; value: string; sub: string; icon: typeof Activity }) {
  return (
    <div className="metric">
      <div className="metric-icon">
        <Icon size={18} />
      </div>
      <div>
        <div className="metric-label">{label}</div>
        <div className="metric-value">{value}</div>
        <div className="metric-sub">{sub}</div>
      </div>
    </div>
  );
}

function LoadableTable<T>({ state, render }: { state: Loadable<T>; render: (data: T) => ReactNode }) {
  if (state.loading) return <Loading label="Loading data" />;
  if (state.error) return <ErrorState message={state.error} />;
  if (!state.data) return <EmptyState label="No data" />;
  return <>{render(state.data)}</>;
}

function SimpleTable({ columns, rows, empty }: { columns: string[]; rows: ReactNode[][]; empty: string }) {
  if (rows.length === 0) return <EmptyState label={empty} />;
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column}>{column}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={rowIndex}>
              {row.map((cell, cellIndex) => (
                <td key={cellIndex}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Loading({ label }: { label: string }) {
  return (
    <div className="state-line">
      <RefreshCw className="spin" size={18} />
      <span>{label}</span>
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="state-line error">
      <AlertTriangle size={18} />
      <span>{message}</span>
    </div>
  );
}

function EmptyState({ label }: { label: string }) {
  return <div className="empty-state">{label}</div>;
}

function statusPill(value: string) {
  const normalized = value.toLowerCase();
  const tone = normalized.includes("fail") || normalized.includes("error") ? "bad" : normalized.includes("warn") ? "warn" : "good";
  return <span className={`pill ${tone}`}>{value || "-"}</span>;
}

async function loadState<T>(setter: (value: Loadable<T>) => void, loader: () => Promise<T>) {
  setter({ loading: true, error: "", data: null });
  try {
    const data = await loader();
    setter({ loading: false, error: "", data });
  } catch (error) {
    setter({ loading: false, error: error instanceof Error ? error.message : "Request failed", data: null });
  }
}

function subtitleFor(view: ViewKey) {
  switch (view) {
    case "overview":
      return "Current system health and recent operational state.";
    case "ops":
      return "Watermarks, task runs, and data quality issues.";
    case "realtime":
      return "Realtime quote service run history and sweep outcomes.";
    case "bestip":
      return "TDX HQ server selection cache and refresh workflow.";
    case "smoke":
      return "Non-destructive provider checks for network and quote decoding.";
  }
}

function formatClock(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, { hour12: false });
}

function formatNumber(value: number) {
  return value.toLocaleString(undefined, { maximumFractionDigits: 3 });
}

function shortID(value: string) {
  if (value.length <= 12) return value;
  return `${value.slice(0, 8)}…${value.slice(-4)}`;
}
