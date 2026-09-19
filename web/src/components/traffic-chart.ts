export type TrafficMetric =
 | "sessions" | "sent" | "received" | "errors"
 | "limit_request_rejected"
 | "limit_connection_rejected"
 | "limit_delayed"
  | "rps"
  | "response_rps"
  | "connections"
  | "requests"
  | "error_rate"
  | "client_error_rate"
  | "server_error_rate"
  | "average_request_time_ms"
  | "average_upstream_header_time_ms"
  | "average_upstream_time_ms";

export interface TrafficSeries {
  key: TrafficMetric;
  label: string;
  color: string;
  unit: string;
  axis?: "left" | "right";
  area?: boolean;
}
