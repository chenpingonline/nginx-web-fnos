const gateway = location.pathname.startsWith("/app/nginx-web") ? "/app/nginx-web" : "";
const apiRoot = `${gateway}/api`;
export async function request<T>(endpoint:string, options:RequestInit={}):Promise<T> {
  const init:RequestInit={credentials:"same-origin",...options}; const headers=new Headers(options.headers);
  if(init.body!==undefined && !(init.body instanceof FormData)) headers.set("Content-Type","application/json");
  if(init.method && !["GET","HEAD"].includes(init.method)) headers.set("X-FnProxy-Request","1");
  init.headers=headers; const response=await fetch(`${apiRoot}${endpoint}`,init); const contentType=response.headers.get("content-type")??"";
  const payload:unknown=contentType.includes("application/json")?await response.json():await response.text();
  if(!response.ok){const message=typeof payload==="object"&&payload&&"error" in payload?String(payload.error):String(payload||`HTTP ${response.status}`);throw new Error(message)}
  return payload as T;
}
export const jsonBody=(value:unknown):string=>JSON.stringify(value);
export const errorMessage=(error:unknown):string=>error instanceof Error?error.message:String(error);
