#  ssh-for-agents

Turn any local folder into an AI-agent-friendly SSH workspace.

`ssh-for-agents` lets local agents such as Codex and Claude Code edit code locally, sync runnable code to a remote SSH server, run commands in the real remote environment, and return logs/results.

## Install

macOS / Linux:

~~~bash
curl -fsSL https://raw.githubusercontent.com/QuelThalasGrace/ssh-for-agents/main/install.sh | sh
~~~

Windows PowerShell:

~~~powershell
iwr -useb https://raw.githubusercontent.com/QuelThalasGrace/ssh-for-agents/main/install.ps1 | iex
~~~

## **Requirements**

- OpenSSH client: `ssh`, `ssh-keygen`, `scp`
- No Python, Go, Node, Bash, rsync, WSL, or Git Bash required for users.

## **What is synchronized?**

The local directory is the source of truth for code.

The remote directory is a runnable code mirror.

Only code and project configuration files are synchronized. Runtime artifacts such as logs, outputs, checkpoints, pids, data, and datasets are not synchronized and do not need to match the local directory.