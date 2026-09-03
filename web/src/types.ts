export type Page =
  | "dashboard"
  | "rules"
  | "upstreams"
  | "certificates"
  | "logs"
  | "revisions"
  | "config"
  | "settings";
export type NginxAction = "start" | "stop" | "reload" | "test";
export type LogType = "error" | "access" | "backend";
export interface RealIPSettings {
  enabled: boolean;
  header: "X-Real-IP" | "X-Forwarded-For" | "proxy_protocol";
  trusted_proxies: string[];
  recursive: boolean;
}
export interface GzipSettings {
  enabled: boolean;
  level: number;
  min_length: number;
  types: string[];
  static: boolean;
  gunzip: boolean;
}
export interface TLSSettings {
  protocols: string[];
  ciphers: string;
  session_cache_mb: number;
  session_timeout_minutes: number;
}
export interface LoggingSettings {
  access_enabled: boolean;
  error_level: string;
  access_buffer_kb: number;
  access_flush_seconds: number;
}
export interface Settings {
  default_http_port: number;
  default_https_port: number;
  revision_limit: number;
  worker_processes: number;
  worker_connections: number;
  worker_rlimit_nofile: number;
  multi_accept: boolean;
  file_aio: boolean;
  real_ip: RealIPSettings;
  gzip: GzipSettings;
  tls: TLSSettings;
  logging: LoggingSettings;
}
export interface RateLimitSettings {
  enabled: boolean;
  requests_per_second: number;
  burst: number;
  no_delay: boolean;
  connections: number;
  download_kbps: number;
}
export interface ProxyRuleInput {
  name: string;
  enabled: boolean;
  listen_port: number;
  domains: string[];
  tls: boolean;
  http2: boolean;
  certificate_id: string;
  upstream_scheme: "http" | "https";
  upstream_host: string;
  upstream_port: number;
  upstream_pool_id: string;
  preserve_host: boolean;
  websocket: boolean;
  streaming: boolean;
  verify_upstream_tls: boolean;
  connect_timeout_seconds: number;
  read_timeout_seconds: number;
  send_timeout_seconds: number;
  client_max_body_mb: number;
  rate_limit: RateLimitSettings;
}
export interface ProxyRule extends ProxyRuleInput {
  id: string;
  created_at: string;
  updated_at: string;
}
export interface CertificateMeta {
  id: string;
  name: string;
  subject: string;
  dns_names: string[];
  ip_addresses: string[];
  serial_number: string;
  not_before: string;
  not_after: string;
  fingerprint: string;
  created_at: string;
}
export interface UpstreamServer {
  host: string;
  port: number;
  weight: number;
  max_fails: number;
  fail_timeout_seconds: number;
  backup: boolean;
  down: boolean;
}
export interface UpstreamPoolInput {
  name: string;
  protocol: "http" | "stream";
  strategy: "round_robin" | "least_conn" | "ip_hash" | "hash" | "random";
  hash_key: string;
  keepalive: number;
  keepalive_requests: number;
  keepalive_time_seconds: number;
  keepalive_timeout_seconds: number;
  servers: UpstreamServer[];
}
export interface UpstreamPool extends UpstreamPoolInput {
  id: string;
  created_at: string;
  updated_at: string;
}
export interface State {
  schema_version: number;
  settings: Settings;
  rules: ProxyRule[];
  certificates: CertificateMeta[];
  upstream_pools: UpstreamPool[];
  dirty: boolean;
  last_applied_at?: string;
  last_apply_message?: string;
  updated_at: string;
}
export interface Revision {
  id: string;
  created_at: string;
  summary: string;
  rule_count: number;
  enabled_count: number;
  state: State;
}
export interface NginxStatus {
  running: boolean;
  pid?: number;
  version: string;
  ports: number[];
  config_path: string;
  last_error?: string;
}
export interface Overview {
  app_version: string;
  nginx_version: string;
  nginx: NginxStatus;
  rule_count: number;
  enabled_count: number;
  certificate_count: number;
  dirty: boolean;
  last_applied_at?: string;
}
export interface GeneratedConfig {
  master: string;
  files: Record<string, string>;
}
export interface ApplyResult {
  action: string;
  message: string;
  output?: string;
}
export interface LogResponse {
  type: LogType;
  lines: string[];
}
export interface CertificateInput {
  name: string;
  certificate: string;
  private_key: string;
}
export interface Toast {
  id: number;
  message: string;
  type: "" | "success" | "error";
}
