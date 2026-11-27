import { YesMemRPC } from "./rpc";

const harmlessCommands = ["grep", "diff", "test", "[", "rg"];

function isHarmlessBashExit(command: string): boolean {
  const cmd = command.split(/\s+/)[0];
  return harmlessCommands.includes(cmd);
}

export function failureLearnHook(rpc: YesMemRPC): Record<string, any> {
  return {
    "tool.execute.after": async (input, output) => {
      const state = (output as any).state;
      if (!state || state.status !== "error") return;

      const tool = input.tool;
      const directory = (input.session as any)?.directory || "";

      if (tool === "bash") {
        const command = ((output as any).args as any)?.command as string;
        if (command && isHarmlessBashExit(command)) return;
      }

      const errorOutput = state.output || state.error || "";
      if (errorOutput.length < 10) return;

      // Phase 1: Learn
      const truncated = errorOutput.length > 200 ? errorOutput.substring(0, 200) : errorOutput;
      // force: this hook reports an actual failure, and the error text is the
      // information. Without it a text that merely shares vocabulary with an
      // existing learning would come back stored=false and be lost here (this
      // caller does not read the answer — the Go hooks store it for the same
      // reason).
      await rpc.call("remember", {
        text: `${tool} error: ${truncated}`,
        category: "gotcha",
        source: "hook_auto_learned",
        project: directory,
        force: true,
      });

      // Phase 2: Assist
      const searchResp = await rpc.call("hybrid_search", {
        query: `${tool} ${truncated.substring(0, 100)}`,
        project: directory,
        limit: 3,
      });
      if (searchResp.ok && searchResp.result?.results?.length > 0) {
        const snippets = searchResp.result.results
          .slice(0, 3)
          .map((r: any) => `- [${r.category}] ${r.content}`)
          .join("\n");
        (output as any).context = `[YesMem Assist] Similar known issues:\n${snippets}`;
      }
    },
  };
}
