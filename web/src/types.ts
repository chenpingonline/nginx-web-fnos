export type Page = "dashboard" | "rules" | "certificates" | "logs" | "revisions" | "config" | "settings";
export type NginxAction = "start" | "stop" | "reload" | "test";
export type LogType = "error" | "access" | "backend";
export interface Settings { default_http_port:number; default_https_port:number; revision_limit:number }
export interface ProxyRuleInput { name:string; enabled:boolean; listen_port:number; domains:string[]; tls:boolean; http2:boolean; certificate_id:string; upstream_scheme:"http"|"https"; upstream_host:string; upstream_port:number; preserve_host:boolean; websocket:boolean; streaming:boolean; verify_upstream_tls:boolean; connect_timeout_seconds:number; read_timeout_seconds:number; send_timeout_seconds:number; client_max_body_mb:number }
export interface ProxyRule extends ProxyRuleInput { id:string; created_at:string; updated_at:string }
export interface CertificateMeta { id:string; name:string; subject:string; dns_names:string[]; ip_addresses:string[]; serial_number:string; not_before:string; not_after:string; fingerprint:string; created_at:string }
export interface State { schema_version:number; settings:Settings; rules:ProxyRule[]; certificates:CertificateMeta[]; dirty:boolean; last_applied_at?:string; last_apply_message?:string; updated_at:string }
export interface Revision { id:string; created_at:string; summary:string; rule_count:number; enabled_count:number; state:State }
export interface NginxStatus { running:boolean; pid?:number; version:string; ports:number[]; config_path:string; last_error?:string }
export interface Overview { app_version:string; nginx_version:string; nginx:NginxStatus; rule_count:number; enabled_count:number; certificate_count:number; dirty:boolean; last_applied_at?:string }
export interface GeneratedConfig { master:string; files:Record<string,string> }
export interface ApplyResult { action:string; message:string; output?:string }
export interface LogResponse { type:LogType; lines:string[] }
export interface CertificateInput { name:string; certificate:string; private_key:string }
export interface Toast { id:number; message:string; type:""|"success"|"error" }
