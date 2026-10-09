package launcher

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// pickDir 弹 Windows 的「浏览文件夹」窗口。
//
// 起始目录和标题走环境变量传,不拼进 PowerShell 脚本,免得路径里的引号出问题。
// 先试 WinForms 的 FolderBrowserDialog(新版,能建新文件夹),它偶尔会报
// "Unable to retrieve the root folder"(用户目录取不到时就会),这时退到
// Shell.Application.BrowseForFolder(旧版树形对话框)。
// 用户取消时返回空字符串,不报错。
func pickDir(ctx context.Context, from, title string) (string, error) {
	if from == "" {
		from = "."
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Sta", "-ExecutionPolicy", "Bypass",
		"-Command", pickScript)
	cmd.Env = append(os.Environ(), "HZ_PICK_START="+from, "HZ_PICK_TITLE="+title)
	prepareCmd(cmd) // 不弹控制台窗口;对话框自己的窗口照常显示
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "", ctx.Err()
	case err != nil:
		// 两种方式都不行:细节进日志,给网页一句能看懂的话
		return "", errors.New(strings.TrimSpace(firstLine(errb.String())))
	}
	return absDir(strings.TrimSpace(out.String())), nil
}

// 成功就把选中的路径打到 stdout(取消则什么都不打);两种方式都失败时打 HZ_ERR: 开头的
// 一行人话到 stdout,详细异常留在 stderr。
const pickScript = `
$ErrorActionPreference = 'Stop'
try {
    Add-Type -AssemblyName System.Windows.Forms
    $d = New-Object System.Windows.Forms.FolderBrowserDialog
    $d.Description = $env:HZ_PICK_TITLE
    $d.ShowNewFolderButton = $true
    if ($env:HZ_PICK_START -and (Test-Path -LiteralPath $env:HZ_PICK_START)) { $d.SelectedPath = $env:HZ_PICK_START }
    if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) }
    exit 0
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
}
try {
    $sh = New-Object -ComObject Shell.Application
    # 0x41 = BIF_NEWDIALOGSTYLE + BIF_RETURNONLYFSDIRS;BIF_EDITBOX 让用户能直接敲路径
    $r = $sh.BrowseForFolder(0, $env:HZ_PICK_TITLE, 0x51, $env:HZ_PICK_START)
    if ($r) { [Console]::Out.Write($r.Self.Path) }
    exit 0
} catch {
    [Console]::Out.Write('HZ_ERR:')
}
exit 1
`

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}
