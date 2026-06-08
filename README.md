#  ssh-for-agents

Turn any local folder into an AI-agent-friendly SSH workspace.

`ssh-for-agents` lets local agents such as Codex and Claude Code edit code locally, sync runnable code to a remote SSH server, run commands in the real remote environment, and return logs/results.

中文：**ssh-for-agents** 可以把任意本地文件夹变成适合 AI Agent 使用的远程 SSH 工作区。Codex、Claude Code 等本地 Agent 可以在本地修改代码，然后自动同步到远程服务器运行，并拿回结果和日志。

### **Features**

- Single binary CLI
- No Python, Go, Node, Bash, rsync, WSL, or Git Bash required for users
- Uses system OpenSSH: `ssh`, `ssh-keygen`, `scp`
- Works with macOS, Linux, and Windows
- Local directory is the source of truth for code
- Remote directory is a runnable code mirror
- Foreground command execution
- Background jobs with logs and PID tracking
- Pull remote code back to local
- Clean local/remote code and test artifacts
- Destroy local and remote project directories with confirmation
- Safe uninstall that never touches SSH config or SSH keys
- Agent-friendly `AGENTS.md` and `CLAUDE.md`

## Install

macOS / Linux:

~~~bash
curl -fsSL https://raw.githubusercontent.com/QuelThalasGrace/ssh-for-agents/main/install.sh | sh
~~~

If `~/.local/bin` is not in your PATH:

~~~bash
export PATH="$HOME/.local/bin:$PATH"
~~~



#### **Direct binary install**

Apple Silicon Mac:

~~~bash
mkdir -p ~/.local/bin
curl -L https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download/sfa-darwin-arm64 -o ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
export PATH="$HOME/.local/bin:$PATH"
~~~



Intel Mac:

~~~bash
mkdir -p ~/.local/bin
curl -L https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download/sfa-darwin-amd64 -o ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
export PATH="$HOME/.local/bin:$PATH"
~~~



Linux amd64:

~~~bash
mkdir -p ~/.local/bin
curl -L https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download/sfa-linux-amd64 -o ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
export PATH="$HOME/.local/bin:$PATH"
~~~



Windows PowerShell:

~~~powershell
iwr -useb https://raw.githubusercontent.com/QuelThalasGrace/ssh-for-agents/main/install.ps1 | iex
~~~

Restart your terminal after installation if `sfa` is not immediately available.

### **Requirements**

Users only need:

ssh
ssh-keygen
scp

No language runtime is required.



### **Quickstart**

Create or enter a local project directory:

~~~bash
mkdir my_project
cd my_project
~~~



Initialize a remote SSH workspace:

~~~bash
sfa init \
  --alias myserver \
  --host 1.2.3.4 \
  --user ubuntu \
  --port 22 \
  --remote-dir /home/ubuntu/my_project
~~~

If the remote server requires password login, type the password only in the terminal prompt. Do not paste passwords into Codex, Claude Code, or chat.

`sfa init` will:

configure SSH alias if needed
generate an SSH key if needed
install the public key if password login is used
create or reuse the remote directory
generate .agent/config.json
generate AGENTS.md and CLAUDE.md
run a language-agnostic shell hello test



### **Keep local and remote directory names consistent**

You can use `--remote-base` instead of `--remote-dir`.

If the local directory is HDCN, then:

~~~bash
sfa init \
  --alias s518 \
  --host 162.105.183.229 \
  --user s514-2 \
  --port 2621 \
  --remote-base /data1/s514-2/project
~~~

will use:

~~~bash
/data1/s514-2/project/HDCN
~~~

The local directory name is preserved exactly, including case.



