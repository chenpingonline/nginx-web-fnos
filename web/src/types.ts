export type Page =
  | "dashboard"
  | "rules"
  | "streams"
  | "upstreams"
  | "certificates"
  | "logs"
  | "revisions"
  | "config"
  | "settings";
export type NginxAction = "start" | "stop" | "reload" | "test";
export type LogType = "error" | "access" | "stream" | "backend";
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
  ocsp_stapling: boolean;
  client_verify: "off" | "on" | "optional" | "optional_no_ca";
  client_ca_file: string;
  client_verify_depth: number;
}
export interface LoggingSettings {
  access_enabled: boolean;
  error_level: string;
  access_buffer_kb: number;
  access_flush_seconds: number;
  custom_format: string;
  rotate_size_mb: number;
  rotate_keep: number;
}
export interface KeyValue { key: string; value: string }
export interface MapDefinition {
  name: string; source: string; variable: string; hostnames: boolean; default: string; entries: KeyValue[];
}
export interface GeoDefinition {
  name: string; source: string; variable: string; default: string; entries: KeyValue[];
}
export interface SplitDefinition {
  name: string; source: string; variable: string; entries: KeyValue[];
}
export interface DynamicRoutingSettings {
  maps: MapDefinition[];
  geos: GeoDefinition[];
  splits: SplitDefinition[];
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
  thread_pool_threads: number;
  thread_pool_queue: number;
  real_ip: RealIPSettings;
  gzip: GzipSettings;
  tls: TLSSettings;
  logging: LoggingSettings;
  routing: DynamicRoutingSettings;
}
export interface RateLimitSettings {
  enabled: boolean;
  requests_per_second: number;
  burst: number;
  no_delay: boolean;
  connections: number;
  download_kbps: number;
}
export interface CacheSettings {
  enabled: boolean;
  keys_zone_mb: number;
  max_size_mb: number;
  inactive_minutes: number;
  valid_seconds: number;
  slice_kb: number;
  use_stale: boolean;
  key: string;
  bypass: string[];
}
export interface RewriteRule {
  pattern: string;
  replacement: string;
  flag: "last" | "break" | "redirect" | "permanent";
}
export interface HeaderSetting {
  name: string;
  value: string;
  always: boolean;
}
export interface LocationSettings {
  backend_type:
    | "proxy"
    | "static"
    | "return"
    | "grpc"
    | "fastcgi"
    | "uwsgi"
    | "scgi"
    | "memcached"
    | "status";
  upstream_scheme: "http" | "https";
  upstream_pool_id: string;
  upstream_host: string;
  upstream_port: number;
  static_path: string;
  static_alias: boolean;
  index_files: string[];
  autoindex: boolean;
  expires: string;
  try_files: string[];
  return_code: number;
  return_target: string;
  redirect_to_https: boolean;
  rewrites: RewriteRule[];
  cache: CacheSettings;
  allow: string[];
  deny: string[];
  request_headers: HeaderSetting[];
  response_headers: HeaderSetting[];
  basic_auth: boolean;
  basic_auth_realm: string;
  basic_auth_file: string;
  auth_request: string;
  secure_link: { enabled: boolean; secret: string; argument: string };
  dav: {
    enabled: boolean;
    methods: string[];
    create_full_put_path: boolean;
    min_delete_depth: number;
  };
  sub_filters: { search: string; replacement: string }[];
  sub_filter_once: boolean;
  sub_filter_types: string[];
  addition_before: string;
  addition_after: string;
  mirror: string;
  mirror_request_body: boolean;
  ssi: boolean;
  valid_referers: string[];
  deny_invalid_referer: boolean;
}
export interface LocationRule {
  id: string;
  name: string;
  enabled: boolean;
  path: string;
  match: "prefix" | "exact" | "regex";
  settings: LocationSettings;
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
  root_location: LocationSettings;
  locations: LocationRule[];
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
export interface SNIRoute {
  server_names: string[];
  upstream_pool_id: string;
  upstream_host: string;
  upstream_port: number;
}
export interface StreamRuleInput {
  name: string;
  enabled: boolean;
  protocol: "tcp" | "udp";
  listen_address: string;
  listen_port: number;
  upstream_pool_id: string;
  upstream_host: string;
  upstream_port: number;
  connect_timeout_seconds: number;
  proxy_timeout_seconds: number;
  udp_responses: number;
  proxy_protocol: boolean;
  accept_proxy_protocol: boolean;
  trusted_proxies: string[];
  tls_mode: "off" | "terminate" | "passthrough";
  certificate_id: string;
  sni_routes: SNIRoute[];
  access_log: boolean;
  allow: string[];
  deny: string[];
  max_connections: number;
}
export interface StreamRule extends StreamRuleInput {
  id: string;
  created_at: string;
  updated_at: string;
}
export interface State {
  schema_version: number;
  settings: Settings;
  rules: ProxyRule[];
  stream_rules: StreamRule[];
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
