export type Health = {
  status: string;
  version: string;
  schema_version: string;
};

export type Watermark = {
  dataset: string;
  asset: string;
  status: string;
  min_watermark?: string;
  max_watermark?: string;
  rows_written: number;
  message: string;
  updated_at: string;
};

export type TaskRun = {
  run_id: string;
  dataset: string;
  task_type: string;
  status: string;
  target_table: string;
  input_path: string;
  input_format: string;
  params: string;
  started_at: string;
  finished_at?: string;
  duration_ms?: number;
  rows_written: number;
  rows_skipped: number;
  error?: string;
  updated_at: string;
};

export type DataQualityIssue = {
  issue_id: string;
  run_id: string;
  dataset: string;
  severity: string;
  issue_type: string;
  market?: string;
  symbol?: string;
  logical_key: string;
  input_path: string;
  input_record_offset?: number;
  observed_at: string;
  message: string;
  details?: string;
};

export type QualityIssueStat = {
  dataset: string;
  severity: string;
  issue_type: string;
  count: number;
};

export type QuoteServiceRun = {
  run_id: string;
  status: string;
  markets: string[];
  symbol_source: string;
  batch_size: number;
  planned_symbols: number;
  planned_batches: number;
  succeeded_batches: number;
  failed_batches: number;
  skipped_batches: number;
  rows_fetched: number;
  started_at: string;
  finished_at?: string;
  duration_ms?: number;
  error?: string;
  updated_at: string;
};

export type ProbeResult = {
  server: string;
  success: boolean;
  latency_ms: number;
  error?: string;
  preferred?: boolean;
};

export type BestIPStatus = {
  cache_path: string;
  generated_at?: string;
  expires_at?: string;
  preferred?: string;
  usable: boolean;
  results: ProbeResult[];
  error?: string;
};

export type Quote = {
  market: string;
  symbol: string;
  price: number;
  last_close: number;
  open: number;
  high: number;
  low: number;
  server_intraday_time: string;
  trade_date?: string;
  quote_time?: string;
  volume: number;
  amount: number;
};

export type ConsoleSummary = {
  health: Health;
  watermarks: Watermark[];
  task_runs: TaskRun[];
  data_quality_issue_counts: QualityIssueStat[];
  quote_service_runs: QuoteServiceRun[];
};

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path);
  return readJSON<T>(response);
}

async function postJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { method: "POST" });
  return readJSON<T>(response);
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = response.statusText;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) {
        message = payload.error;
      }
    } catch {
      // Keep status text when the response is not JSON.
    }
    throw new Error(message);
  }
  return (await response.json()) as T;
}

export const api = {
  summary: (limit = 20) => getJSON<ConsoleSummary>(`/api/console/summary?limit=${limit}`),
  watermarks: (limit = 50) => getJSON<Watermark[]>(`/api/console/watermarks?limit=${limit}`),
  taskRuns: (limit = 50) => getJSON<TaskRun[]>(`/api/console/task-runs?limit=${limit}`),
  dataQualityIssues: (limit = 50) => getJSON<DataQualityIssue[]>(`/api/console/data-quality-issues?limit=${limit}`),
  quoteRuns: (limit = 50) => getJSON<QuoteServiceRun[]>(`/api/console/quote-service/runs?limit=${limit}`),
  bestip: () => getJSON<BestIPStatus>("/api/console/bestip"),
  refreshBestip: (servers: string, maxAge: string) => {
    const params = new URLSearchParams();
    if (servers.trim()) params.set("server", servers);
    if (maxAge.trim()) params.set("max-age", maxAge);
    return postJSON<BestIPStatus>(`/api/console/bestip/refresh?${params.toString()}`);
  },
  probe: (servers: string) => {
    const params = new URLSearchParams();
    if (servers.trim()) params.set("server", servers);
    return getJSON<{ results: ProbeResult[] }>(`/api/console/tdx/hq/probe?${params.toString()}`);
  },
  quoteSmoke: (symbols: string) => {
    const params = new URLSearchParams();
    params.set("symbol", symbols);
    return getJSON<{ quotes: Quote[] }>(`/api/console/tdx/hq/quotes?${params.toString()}`);
  }
};
