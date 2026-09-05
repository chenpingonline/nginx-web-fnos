export interface LogToken { text: string; tone?: string }

// Preserve the original text. Tokens are escaped by Vue interpolation.
export function highlightLog(line: string): LogToken[] {
  const tokens: LogToken[] = [];
  const pattern = /\[(?:\d{2}\/[^\]]+|\d{4}[^\]]+)\]|"(?:GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS|CONNECT|TRACE)\s[^"\r\n]*"\s+[1-5]\d{2}\b|\b(?:TCP|UDP)\s+[1-5]\d{2}\b|\[(?:debug|info|notice|warn|error|crit|alert|emerg)\]|\b(?:level|status|request_time|upstream_time|session_time|host|upstream)=(?:"[^"\r\n]*"|[^\s]+)|读取失败：/gi;
  let offset = 0;
  const push = (text: string, tone?: string) => { if (text) tokens.push({ text, tone }); };
  const statusTone = (status: string) => Number(status) >= 500 ? "error" : Number(status) >= 400 ? "warning" : Number(status) >= 300 ? "redirect" : "success";
  const levelTone = (level: string) => /error|crit|alert|emerg|fatal/i.test(level) ? "error" : /warn/i.test(level) ? "warning" : /debug/i.test(level) ? "muted" : "info";
  for (const match of line.matchAll(pattern)) {
    const value = match[0];
    const start = match.index!;
    push(line.slice(offset, start));
    if (/^("|TCP\s|UDP\s)/i.test(value)) {
      const statusStart = value.length - 3;
      const methodStart = value.startsWith('"') ? 1 : 0;
      const methodEnd = value.indexOf(" ");
      push(value.slice(0, methodStart));
      push(value.slice(methodStart, methodEnd), "method");
      push(value.slice(methodEnd, statusStart));
      push(value.slice(statusStart), statusTone(value.slice(statusStart)));
    } else if (/^\[\d/.test(value)) {
      push(value, "muted");
    } else if (value.startsWith("[") || /^level=/i.test(value)) {
      push(value, levelTone(value));
    } else if (/^status=/i.test(value)) {
      const status = value.slice(7).replaceAll('"', "");
      push(value, /^[1-5]\d{2}$/.test(status) ? statusTone(status) : undefined);
    } else if (/^(request_time|upstream_time|session_time)=/i.test(value)) {
      const seconds = value.slice(value.indexOf("=") + 1).replaceAll('"', "").split(/[, :]+/).map(Number);
      push(value, seconds.some((n) => n >= 1) ? "warning" : "muted");
    } else {
      push(value, value.startsWith("读取失败") ? "error" : "host");
    }
    offset = start + value.length;
  }
  push(line.slice(offset));
  return tokens;
}

export interface SearchLogToken extends LogToken { matchIndex?: number }

export function searchLogLines(lines: LogToken[][], query: string) {
  let count = 0;
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const pattern = query ? new RegExp(escaped, "gi") : null;
  const result = lines.map((tokens): SearchLogToken[] => {
    if (!pattern) return tokens;
    const text = tokens.map(token => token.text).join("");
    const matches = Array.from(text.matchAll(pattern), match => ({
      start: match.index!, end: match.index! + match[0].length, index: count++,
    }));
    const parts: SearchLogToken[] = [];
    let offset = 0;
    let matchCursor = 0;
    for (const token of tokens) {
      const end = offset + token.text.length;
      let cursor = offset;
      while (matchCursor < matches.length && matches[matchCursor].end <= cursor) matchCursor++;
      while (cursor < end) {
        const match = matches[matchCursor];
        const inside = match && cursor >= match.start && cursor < match.end;
        const boundary = Math.min(end, match ? inside ? match.end : match.start : end);
        parts.push({ ...token, text: token.text.slice(cursor - offset, boundary - offset), matchIndex: inside ? match.index : undefined });
        cursor = boundary;
        if (match && cursor >= match.end) matchCursor++;
      }
      offset = end;
    }
    return parts;
  });
  return { lines: result, count };
}
