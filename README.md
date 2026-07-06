# ssh-for-agents

把任意本地文件夹变成适合 AI Agent 使用的 SSH 工作区。
Turn any local folder into an AI-agent-friendly SSH workspace.

`ssh-for-agents` 让 Codex、Claude Code 等本地 Agent 可以在本地修改代码，按需同步到远程 SSH 服务器运行，并取回日志与结果。
`ssh-for-agents` lets local agents such as Codex and Claude Code edit code locally, sync runnable code to a remote SSH server when needed, run commands in the real remote environment, and return logs/results.

## 功能 / Features

- 单文件 CLI。 / Single binary CLI.
- 用户不需要安装 Python、Go、Node、Bash、rsync、WSL 或 Git Bash。 / Users do not need Python, Go, Node, Bash, rsync, WSL, or Git Bash.
- 使用系统 OpenSSH 工具：`ssh`、`ssh-keygen`、`scp`。 / Uses system OpenSSH tools: `ssh`, `ssh-keygen`, `scp`.
- 支持 macOS、Linux 和 Windows。 / Works with macOS, Linux, and Windows.
- 本地目录是代码的事实来源。 / The local directory is the source of truth for code.
- 远程目录是可运行的代码镜像。 / The remote directory is a runnable code mirror.
- `.env` 和 `.env.*` 等环境变量文件永不同步。 / Environment files such as `.env` and `.env.*` are never synced.
- 支持显式同步和指定文件双向同步，避免每次运行都传输大项目。 / Supports explicit sync and selected-file sync in both directions so large projects do not need to transfer every file before every run.
- 支持前台命令执行。 / Supports foreground command execution.
- 支持后台任务、日志和 PID 跟踪。 / Supports background jobs with logs and PID tracking.
- 支持把远程代码拉回本地。 / Supports pulling remote code back to local.
- 支持清理本地和远程的代码与测试产物。 / Supports cleaning local and remote code and test artifacts.
- 删除本地与远程项目目录前需要确认。 / Destroying local and remote project directories requires confirmation.
- 安全卸载不会修改 SSH config 或 SSH keys。 / Safe uninstall never touches SSH config or SSH keys.
- 支持在线更新 `sfa` 并刷新 `SFA.md`。 / Supports online `sfa` upgrades and `SFA.md` refreshes.
- 自动生成适合 Agent 阅读的 `SFA.md`，并为 Codex 和 Claude Code 配置轻量入口。 / Generates agent-friendly `SFA.md` and lightweight Codex and Claude Code entry points.

## 安装 / Install

### macOS / Linux

推荐使用一行安装脚本。
Use the one-line installer:

~~~bash
curl -fsSL https://raw.githubusercontent.com/QuelThalasGrace/ssh-for-agents/main/install.sh | sh
~~~

如果安装后找不到 `sfa`，请把 `~/.local/bin` 永久加入 PATH。
If `sfa` is not found after installation, add `~/.local/bin` to PATH permanently.

zsh，现代 macOS 默认使用：
For zsh, which is the default shell on modern macOS:

~~~bash
grep -qxF 'export PATH="$HOME/.local/bin:$PATH"' ~/.zshrc || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
~~~

bash：
For bash:

~~~bash
grep -qxF 'export PATH="$HOME/.local/bin:$PATH"' ~/.bashrc || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
~~~

然后验证安装。
Then verify the installation:

~~~bash
sfa doctor
~~~

### 直接安装二进制 / Direct binary install

Apple Silicon Mac：

~~~bash
mkdir -p ~/.local/bin
curl -L https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download/sfa-darwin-arm64 -o ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
codesign --force --sign - ~/.local/bin/sfa
~/.local/bin/sfa doctor
~~~

Intel Mac：

~~~bash
mkdir -p ~/.local/bin
curl -L https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download/sfa-darwin-amd64 -o ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
codesign --force --sign - ~/.local/bin/sfa
~/.local/bin/sfa doctor
~~~

Linux amd64：

~~~bash
mkdir -p ~/.local/bin
curl -L https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download/sfa-linux-amd64 -o ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
~/.local/bin/sfa doctor
~~~

如果希望直接输入 `sfa` 而不是 `~/.local/bin/sfa`，请使用上面的永久 PATH 配置。
Use the permanent PATH setup above if you want to run `sfa` without typing `~/.local/bin/sfa`.

### macOS 本地源码安装 / Local source install on macOS

如果你从源码目录本地编译安装，macOS 可能会因为临时签名状态直接终止新二进制并显示 `zsh: killed`。安装后重新做一次 ad-hoc signing 即可。
If you build and install from the source directory, macOS may terminate the new binary with `zsh: killed` because of its temporary signing state. Re-sign it locally after installation:

~~~bash
go build -o sfa ./cmd/sfa
mkdir -p ~/.local/bin
cp ./sfa ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
codesign --force --sign - ~/.local/bin/sfa
sfa version
~~~

### Windows PowerShell

推荐使用安装脚本。
Use the installer:

~~~powershell
iwr -useb https://raw.githubusercontent.com/QuelThalasGrace/ssh-for-agents/main/install.ps1 | iex
~~~

如果安装后不能立即使用 `sfa`，请重启终端。
Restart your terminal after installation if `sfa` is not immediately available.

### 从 GitHub 手动本地安装 / Manual local install from GitHub

如果一行安装脚本因为 `curl`、`wget`、shell 策略或网络限制失败，可以使用这个方案。
Use this if the one-line installer fails because `curl`, `wget`, shell policy, or network restrictions block it.

1. 打开最新 release 页面。
   Open the latest release page:

   https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest

2. 在 Assets 中下载匹配平台的预编译二进制。
   Download the matching prebuilt binary from Assets:

~~~text
Apple Silicon Mac: sfa-darwin-arm64
Intel Mac:         sfa-darwin-amd64
Linux amd64:       sfa-linux-amd64
Linux arm64:       sfa-linux-arm64
Windows amd64:     sfa-windows-amd64.exe
Windows arm64:     sfa-windows-arm64.exe
~~~

普通用户不要使用 GitHub 的 `Source code (zip)` 作为安装包；它包含源码，不包含预编译的 `sfa`。
Do not use GitHub `Source code (zip)` for normal installation. It contains source files, not the prebuilt `sfa` binary.

如果浏览器下载得到的是 zip 文件，请先解压，再移动解压出的 `sfa-*` 文件。
If the browser downloads a zip file, unzip it first and move the extracted `sfa-*` file.

3. 在 macOS / Linux 本地安装下载的文件。
   Install the downloaded file locally on macOS / Linux:

~~~bash
mkdir -p ~/.local/bin
mv ~/Downloads/sfa-darwin-arm64 ~/.local/bin/sfa
chmod +x ~/.local/bin/sfa
xattr -dr com.apple.quarantine ~/.local/bin/sfa 2>/dev/null || true
codesign --force --sign - ~/.local/bin/sfa
~/.local/bin/sfa doctor
~~~

请把 `sfa-darwin-arm64` 替换为你实际下载的文件名。
Replace `sfa-darwin-arm64` with the file you downloaded.

如果 macOS 执行时显示 `zsh: killed`，请再次运行上面的 `xattr` 和 `codesign` 两行。
If macOS prints `zsh: killed` when executing `sfa`, run the `xattr` and `codesign` lines above again.

4. 在 Windows PowerShell 本地安装下载的文件。
   Install the downloaded file locally on Windows PowerShell:

~~~powershell
New-Item -ItemType Directory -Force -Path "$HOME\.ssh-for-agents\bin" | Out-Null
Move-Item "$HOME\Downloads\sfa-windows-amd64.exe" "$HOME\.ssh-for-agents\bin\sfa.exe" -Force
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$HOME\.ssh-for-agents\bin", "User")
~~~

重启 PowerShell，然后验证安装。
Restart PowerShell, then verify the installation:

~~~powershell
sfa doctor
~~~

## 依赖要求 / Requirements

用户只需要系统中存在以下工具。
Users only need these tools on the system:

~~~text
ssh
ssh-keygen
scp
~~~

不需要任何语言运行时。
No language runtime is required.

全量同步会优先使用远端的 `tar` 解压归档；如果远端没有 `tar`，会自动退回逐文件 `scp` 同步。
Full sync prefers remote `tar` to extract an archive. If remote `tar` is unavailable, it falls back to per-file `scp` sync.

## 快速开始 / Quickstart

创建或进入一个本地项目目录。
Create or enter a local project directory:

~~~bash
mkdir my_project
cd my_project
~~~

初始化远程 SSH 工作区。
Initialize a remote SSH workspace:

~~~bash
sfa init myserver
~~~

这里的 `myserver` 是 `~/.ssh/config` 中已经存在且可以免密登录的 `Host` 名称。默认远端目录是服务器 `$HOME/<当前本地目录名>`。
Here `myserver` is an existing passwordless `Host` entry in `~/.ssh/config`. The default remote directory is `$HOME/<current-local-directory-name>` on the server.

如果远程服务器需要密码登录，请只在终端提示中输入密码，不要把密码粘贴到 Codex、Claude Code 或聊天窗口。
If the remote server requires password login, type the password only in the terminal prompt. Do not paste passwords into Codex, Claude Code, or chat.

`sfa init` 会执行以下操作。
`sfa init` will:

- 复用已有 SSH config Host，或按需配置 SSH alias。 / Reuse an existing SSH config Host, or configure an SSH alias if needed.
- 使用完整参数初始化时，按需生成 SSH key。 / Generate an SSH key if needed when using the full init form.
- 使用完整参数初始化且需要密码登录时，会安装 public key。 / Install the public key when using the full init form and password login is needed.
- 创建或复用远程目录。 / Create or reuse the remote directory.
- 生成 `.agent/config.json`。 / Generate `.agent/config.json`.
- 生成 `SFA.md`。 / Generate `SFA.md`.
- 配置 `.codex/config.toml` 让 Codex 识别 `SFA.md`。 / Configure `.codex/config.toml` so Codex can discover `SFA.md`.
- 配置 `.claude/CLAUDE.md` 让 Claude Code 导入 `SFA.md`。 / Configure `.claude/CLAUDE.md` so Claude Code imports `SFA.md`.
- 运行一个不上传项目文件的远程 shell hello test。 / Run a remote shell hello test without uploading project files.

## 保持本地和远程目录名一致 / Keep local and remote directory names consistent

可以使用 `--remote-base` 代替 `--remote-dir`。
You can use `--remote-base` instead of `--remote-dir`.

如果本地目录名是 `HDCN`，例如运行：
If the local directory is `HDCN`, for example:

~~~bash
sfa init s518 --remote-base /data1/s514-2/project
~~~

远程目录会是：
The remote directory will be:

~~~bash
/data1/s514-2/project/HDCN
~~~

本地目录名会被完整保留，包括大小写。
The local directory name is preserved exactly, including case.

如果想完全指定远端目录，可以使用 `--remote-dir`。
Use `--remote-dir` to specify the exact remote directory:

~~~bash
sfa init s518 --remote-dir /data1/s514-2/project/HDCN
~~~

如果还没有可用的 SSH config Host，仍然可以使用完整参数让 `sfa` 配置 SSH alias 和 key。
If you do not already have a usable SSH config Host, use the full form so `sfa` can configure the SSH alias and key:

~~~bash
sfa init \
  --alias myserver \
  --host 1.2.3.4 \
  --user ubuntu \
  --port 22 \
  --remote-dir /home/ubuntu/my_project
~~~

## 同步与运行 / Sync and run

修改代码后，先把本地文件同步到远程代码镜像。
After editing code, sync local files to the remote code mirror first:

~~~bash
sfa sync
~~~

全量 `sfa sync` 会把可同步文件打成归档后上传并在远端解压，比逐文件传输更适合大项目。
Full `sfa sync` uploads syncable files as an archive and extracts it remotely, which is faster than per-file transfer for larger projects.

如果只改了少量文件，可以只同步这些文件。
If only a few files changed, sync only those files:

~~~bash
sfa sync src/train.py
sfa sync src/train.py src/config.py
~~~

默认情况下，`sfa run` 和 `sfa bg-run` 只运行远程命令，不会自动同步。
By default, `sfa run` and `sfa bg-run` only run remote commands and do not sync automatically.

~~~bash
sfa run "python train.py"
sfa bg-run train "python train.py"
~~~

如果希望一次性同步并运行，可以使用 `--sync`。
Use `--sync` to sync and run in one step:

~~~bash
sfa run --sync "python train.py"
sfa bg-run --sync train "python train.py"
~~~

同步和拉取默认启用 SSH connection reuse 来降低多次 `ssh` / `scp` 的连接延迟。如果服务器或跳板环境不兼容，可以关闭：
Sync and pull enable SSH connection reuse by default to reduce repeated `ssh` / `scp` connection latency. Disable it if a server or jump-host setup is incompatible:

~~~bash
sfa sync --no-ssh-reuse
sfa pull --no-ssh-reuse
SFA_SSH_REUSE=0 sfa run --sync "python train.py"
~~~

`sfa pull` 会把远程代码镜像文件复制回本地目录。
`sfa pull` copies remote code mirror files back to the local directory.

~~~bash
sfa pull
~~~

如果只需要取回少量远端文件，可以只拉这些文件。
If only a few remote files are needed, pull only those files:

~~~bash
sfa pull src/generated.py
sfa pull src/train.py src/config.py
~~~

## 更新 sfa / Upgrade sfa

在线更新当前 `sfa` 二进制，并在当前目录已初始化时刷新 `SFA.md` 和 Agent 入口文件。
Upgrade the current `sfa` binary online, and refresh `SFA.md` and agent entry files when the current directory is initialized:

~~~bash
sfa upgrade
~~~

默认从 GitHub 最新 release 下载。可以用 `SFA_BASE_URL` 指向镜像或测试发布目录。
By default, this downloads from the latest GitHub release. Use `SFA_BASE_URL` to point at a mirror or test release directory:

~~~bash
SFA_BASE_URL=https://example.com/sfa sfa upgrade
~~~

## 同步排除规则 / Sync exclusions

两个方向都会跳过环境变量文件和运行时产物。文件名为 `.env` 或以 `.env.` 开头的文件不会同步，例如：
Both directions skip environment files and runtime artifacts. Files named `.env` or starting with `.env.` are not synchronized, including examples such as:

~~~text
.env
.env.local
.env.production
.env.development.local
~~~

以下目录无论出现在任何层级都会被跳过。
The following directories are skipped wherever they appear:

~~~text
.agent/
.codex/
.claude/
.git/
.venv/
__pycache__/
bin/
obj/
logs/
outputs/
checkpoints/
remote_logs/
pids/
data/
datasets/
~~~

以下文件无论出现在任何层级都会被跳过。
The following files are skipped wherever they appear:

~~~text
AGENTS.md
CLAUDE.md
PROJECT_CONTEXT.md
SFA.md
.DS_Store
*.pyc
~~~

`sfa` 同步时不会读取 `.gitignore`。
`sfa` does not read `.gitignore` during sync.

## 用例：让 Agent 配置远程 conda Python 环境 / Example: configure a remote conda Python environment with an agent

执行 `sfa init` 后，项目目录中会包含 `SFA.md`、`.agent/config.json`、`.codex/config.toml` 和 `.claude/CLAUDE.md`，并且 `sfa` 可以在远程服务器执行安装与配置命令。你可以让本地 Agent 帮你准备远程 Python 运行环境。
After `sfa init`, the project directory contains `SFA.md`, `.agent/config.json`, `.codex/config.toml`, and `.claude/CLAUDE.md`, and the `sfa` command can run setup commands on the remote server. You can ask a local agent to prepare the remote Python runtime for you.

可以这样对 Agent 提需求。
Example user prompt to the agent:

~~~text
请你先阅读 SFA.md，然后帮我在远程服务器使用 miniconda（如果没有就先安装 miniconda）新建一个名为 trading 的 python 版本大于 3.11 的 python 环境，并使用 sfa 中配置 python 环境相关的命令将默认环境设定为 trading。
~~~

Agent 应该使用 `sfa run` 和 `sfa env` 完成远程配置。
The agent should then work through the remote setup with `sfa run` and `sfa env`:

1. 阅读 `SFA.md`，确认远程镜像目录和当前运行时配置。
   Read `SFA.md` and confirm the remote mirror and current runtime configuration.
2. 检查远程服务器是否已经存在 conda。
   Check whether conda already exists on the remote server.
3. 如果没有 conda，就在远程服务器安装 Miniconda，例如安装到 `~/miniconda3`。
   If conda is missing, install Miniconda on the remote server, for example under `~/miniconda3`.
4. 创建需要的 conda 环境，例如创建 Python 版本大于 3.11 的 `trading` 环境。
   Create the requested conda environment, for example `trading`, with Python newer than 3.11.
5. 确保远程 shell 能找到 conda；如有需要，把 `~/miniconda3/bin` 加入远程用户的 shell profile。
   Make sure the remote shell can find conda. If needed, add `~/miniconda3/bin` to the remote user's shell profile.
6. 为当前项目配置默认运行环境。
   Configure the default runtime for this project:

~~~bash
sfa env set python conda trading
~~~

7. 验证配置。
   Verify the configuration:

~~~bash
sfa env show
sfa run 'python --version && echo $CONDA_DEFAULT_ENV'
~~~

配置成功后，`.agent/config.json` 中应该出现类似下面的 conda activation prefix。
A successful setup should show that `.agent/config.json` now has a conda activation prefix similar to:

~~~text
. '/home/ubuntu/miniconda3/etc/profile.d/conda.sh' && conda activate 'trading'
~~~

之后，本项目中的所有 `sfa run` 和 `sfa bg-run` 命令都会默认在 `trading` 环境中执行。
After this, all `sfa run` and `sfa bg-run` commands for the project run in the `trading` environment by default.
