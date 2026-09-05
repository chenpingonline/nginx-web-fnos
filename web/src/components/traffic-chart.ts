export type TrafficMetric =
  | "rps"
  | "response_rps"
  | "connections"
  | "error_rate"
  | "client_error_rate"
  | "server_error_rate";

export interface TrafficSeries {
  key: TrafficMetric;
  label: string;
  color: string;
  unit: string;
  axis?: "left" | "right";
  area?: boolean;
}
