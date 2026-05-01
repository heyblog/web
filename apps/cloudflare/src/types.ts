export interface WorkerBindings {
  WORKER_CALLBACK_SECRET?: string;
}

export interface CheckRequestBody {
  url: string;
  timeout_ms?: number;
}

export interface RSSFetchRequestBody {
  feed_url: string;
  timeout_ms?: number;
}

export interface CheckResponseData {
  result: string;
  status_code: number;
  response_time_ms: number;
  duration_ms: number;
  final_url: string;
  content_verified: boolean;
  message: string;
}

export interface RSSFetchResponseData {
  result: string;
  article_count: number;
  final_url: string;
  content_type: string;
  content: string;
  message: string;
}
