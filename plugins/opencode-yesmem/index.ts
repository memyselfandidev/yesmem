import { YesMemRPC } from "./rpc";
import { codeNavHook } from "./code_nav";
import { failureLearnHook } from "./failure_learn";
import { autoResolveHook } from "./auto_resolve";
import { idleReminderHook } from "./idle_reminder";
import { ruleGuardHook } from "./rule_guard";

export const YesMemPlugin = async (ctx: any) => {
  const rpc = new YesMemRPC();
  const directory = ctx.directory || process.env.PWD || "";
  const V = 5; // bump to bust Bun module cache

  // v5: code_nav opencode section restored + error rephrase
  if (directory) {
    rpc.call("search_code_index", {
      pattern: "func",
      kind: "function",
      project: directory,
      limit: 1,
    }).catch(() => {});
  }

  const nv = codeNavHook(rpc, directory);
  const grd = ruleGuardHook(directory);
  const fl = failureLearnHook(rpc);
  const ar = autoResolveHook(rpc);
  const ir = idleReminderHook(rpc);

  // Compose: both need tool.execute.before — code_nav blocks first, then rule_guard
  async function composedBefore(input: any, output: any) {
    try { await nv["tool.execute.before"]?.(input, output); } catch (e) { throw e; }
    try { await grd["tool.execute.before"]?.(input, output); } catch (e) { throw e; }
  }

  // Compose: all three need tool.execute.after — rule_guard injects, failure_learn tracks, auto_resolve resolves
  async function composedAfter(input: any, output: any) {
    try { await grd["tool.execute.after"]?.(input, output); } catch {}
    try { await fl["tool.execute.after"]?.(input, output); } catch {}
    try { await ar["tool.execute.after"]?.(input, output); } catch {}
  }

  return {
    ...grd,              // chat.params, etc.
    ...nv,               // any future code_nav hooks
    "tool.execute.before": composedBefore,
    "tool.execute.after": composedAfter,
    ...ir,               // message.updated
    "shell.env": async (_input: any, output: any) => {
      output.env.YESMEM_SOCKET = `${process.env.HOME || "/home/chief"}/.claude/yesmem/daemon.sock`;
      output.env.YESMEM_SOURCE_AGENT = "opencode";
    },
    "session.created": async (input: any) => {
      try {
        const session = input.session;
        if (!session?.id) return;
        const sid = `opencode:${session.id}`;
        const verbose = process.env.YESMEM_VERBOSE === "1";

        rpc.call("register_pid", {
          session_id: sid,
          pid: process.pid,
          source_agent: "opencode",
        }).catch(() => {});

        if (verbose) {
          console.log(`[yesmem] session.created: ${sid}, pid=${process.pid}`);
        }
      } catch (e: any) {
        if (process.env.YESMEM_VERBOSE === "1") {
          console.error("[yesmem] session.created failed:", e?.message || String(e));
        }
      }
    },
  };
};

export default YesMemPlugin;
