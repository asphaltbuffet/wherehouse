Pick up where a previous agent session that stopped.

> **Project VCS**: See `.claude/project-config.md` → Version Control for VCS commands.

Run these steps to understand the current state before doing anything else:

1. **Unfinished sessions**: `fd -e json state ai-docs -X jq -r 'select(.status=="active") | "\(.session_id):\(.phase)"'`
2. **Check workspaces**: `jj workspace list` — identify workspace you're in and what others exist
3. **Check jj history**: `jj log -n20` — see what was committed recently
4. **Current state**: `jj status` — see what was most recently changed
5. **Read conversation summary**: If the session includes a conversation summary, read it carefully to identify pending tasks or decisions already made.

After gathering context, ask the user what to work on next (even if it seems obvious).
