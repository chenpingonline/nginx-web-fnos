export function formatDate(value?:string,dateOnly=false):string{if(!value)return"—";const date=new Date(value);if(Number.isNaN(date.getTime()))return value;return new Intl.DateTimeFormat("zh-CN",dateOnly?{year:"numeric",month:"2-digit",day:"2-digit"}:{year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hour12:false}).format(date)}
export const formatHost=(host=""):string=>host.includes(":")&&!host.startsWith("[")?`[${host}]`:host;
export const shortFingerprint=(value=""):string=>value.length>23?`${value.slice(0,17)}…${value.slice(-5)}`:value;
