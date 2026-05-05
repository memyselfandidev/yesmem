---
name: proxy_health
description: "Check proxy and daemon health: API errors (upstream, overload, rate limit), fork call stats (count, tokens, learnings), keepalive ping analysis (cache_write threshold), general errors (daemon timeouts, restarts). Pure shell parsing, no LLM needed. Persists reports to cap_store."
version: 72
tags: [health, monitoring, proxy, daemon, scheduled, keepalive, fork]
requires: [store]
scope: user
tested: true
auto_active: true
---

## Purpose

Check proxy and daemon health: API errors (upstream, overload, rate limit), fork call stats (count, tokens, learnings), keepalive ping analysis (cache_write threshold), general errors (daemon timeouts, restarts). Pure shell parsing, no LLM needed. Persists reports to cap_store.

## Scripts

### proxy_health
kind: tool

```javascript
async ({hours, alert, project}) => {
  hours = hours || 1;
  alert = alert !== false;
  project = project || 'yesmem';
  const since = `${hours} hour${hours>1?'s':''} ago`;
  const plog = `journalctl --user -u yesmem-proxy --since "${since}" --no-pager 2>&1`;

  const [apiRaw, forkRaw, forkResultRaw, kaRaw, restarts, errRaw] = await Promise.all([
    sh(`${plog} | grep -iE "upstream error|overload|rate.limit|status.*(502|503|429|500)" | head -50`, 15000),
    sh(`${plog} | grep "fork extract_and_evaluate" | head -50`, 15000),
    sh(`${plog} | grep "\\[fork\\] result:" | head -50`, 15000),
    sh(`${plog} | grep "\\[keepalive\\] ping" | head -50`, 15000),
    sh(`${plog} | grep -c "listening on" || echo 0`, 10000),
    sh(`${plog} | grep -iE "ERR|panic|fatal|refused|timeout" | grep -viE "REWRITE|fork|associative|keepalive" | head -30`, 15000)
  ]);

  const apiLines = apiRaw.trim().split('\n').filter(l => l.trim());
  const apiErrors = {total: apiLines.length, by_type: {}};
  for (const l of apiLines) {
    const type = l.includes('context canceled') ? 'context_canceled' :
                 l.includes('closed network') ? 'connection_closed' :
                 l.includes('overload') ? 'overloaded' :
                 l.includes('rate') ? 'rate_limited' :
                 l.includes('502') ? 'bad_gateway' :
                 l.includes('503') ? 'service_unavailable' :
                 l.includes('429') ? 'rate_limited' :
                 l.includes('500') ? 'internal_server_error' : 'other';
    apiErrors.by_type[type] = (apiErrors.by_type[type] || 0) + 1;
  }

  const forkLines = forkRaw.trim().split('\n').filter(l => l.trim());
  const forkResults = forkResultRaw.trim().split('\n').filter(l => l.trim());
  let totalIn=0, totalCached=0, totalOut=0, totalLearnings=0, totalEvals=0;
  for (const l of forkLines) {
    const m = l.match(/(\d+)\s+in\s*\/\s*(\d+)\s+cached\s*\/\s*(\d+)\s+out/);
    if (m) { totalIn += +m[1]; totalCached += +m[2]; totalOut += +m[3]; }
  }
  for (const l of forkResults) {
    const m = l.match(/(\d+)\s+learnings?/); if (m) totalLearnings += +m[1];
    const e = l.match(/(\d+)\s+evaluations?/); if (e) totalEvals += +e[1];
  }
  const forks = {count: forkLines.length, tokens: {in: totalIn, cached: totalCached, out: totalOut}, learnings: totalLearnings, evaluations: totalEvals};

  const kaLines = kaRaw.trim().split('\n').filter(l => l.trim());
  let kaHealthy=0, kaOver1k=0;
  const kaIssues = [];
  for (const l of kaLines) {
    const cw = l.match(/cache_write=(\d+)/);
    if (cw) { const w = +cw[1]; if (w < 1000) kaHealthy++; else { kaOver1k++; kaIssues.push(`cache_write=${w} in: ${l.slice(l.indexOf('[keepalive]'), l.indexOf('[keepalive]')+80)}`); } }
  }
  const keepalive = {total: kaLines.length, healthy: kaHealthy, cache_write_over_1k: kaOver1k, issues: kaIssues.slice(0,5)};

  const errLines = errRaw.trim().split('\n').filter(l => l.trim());
  const daemonTimeouts = errLines.filter(l => l.includes('i/o timeout')).length;
  const general = {daemon_timeouts: daemonTimeouts, other_errors: errLines.length - daemonTimeouts, proxy_restarts: parseInt(restarts.trim()) || 0};

  let status = 'healthy';
  if (apiErrors.total > 5 || daemonTimeouts > 3 || kaOver1k > 0 || general.proxy_restarts > 2) status = 'degraded';
  if (apiErrors.by_type.overloaded > 0 || apiErrors.by_type.rate_limited > 3 || general.proxy_restarts > 5) status = 'critical';
  if (keepalive.total === 0 && hours >= 1) status = 'degraded';

  const ts = new Date().toISOString().slice(0,19);
  const result = {timestamp: ts, status, hours, api_errors: apiErrors, forks, keepalive, general};

  await mcp__yesmem__cap_store({capability:'proxy_health',action:'create_table',table:'reports',columns:JSON.stringify([{name:'timestamp',type:'TEXT'},{name:'status',type:'TEXT'},{name:'summary',type:'TEXT'},{name:'issues_json',type:'TEXT'},{name:'hours',type:'INTEGER'}])});
  const summary = `${status}: ${apiErrors.total} API err, ${forks.count} forks (${totalLearnings} learnings), ${keepalive.total} pings (${kaOver1k} over 1k cw), ${daemonTimeouts} daemon timeouts`;
  await mcp__yesmem__cap_store({capability:'proxy_health',action:'upsert',table:'reports',data:JSON.stringify({timestamp:ts, status, summary, issues_json: JSON.stringify(result), hours})});

  if (alert && status !== 'healthy') {
    const lines = [`PROXY HEALTH [${status.toUpperCase()}] (${since}):`, summary];
    if (kaOver1k > 0) lines.push(`${kaOver1k} keepalive pings with cache_write >= 1k`);
    if (apiErrors.total > 0) lines.push(`API errors: ${JSON.stringify(apiErrors.by_type)}`);
    await mcp__yesmem__broadcast({project, content: lines.join('\n')});
  }

  return result;
}
```

## Database

```sql
CREATE TABLE cap_proxy_health__reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT,
  status TEXT,
  summary TEXT,
  issues_json TEXT,
  hours INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```
