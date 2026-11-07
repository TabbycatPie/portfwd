package main

// 界面语言：中文 / English。托盘右键「Language / 语言」切换，存进配置的 lang 字段。
// 所有界面文案都从这里取，T("key") 或 T("key", args...)（带格式化）。

import "fmt"

var langEN bool // false = 中文

type tr struct{ zh, en string }

var dict = map[string]tr{
	// 主窗口
	"app.title":    {"端口转发", "PortFwd"},
	"subtitle":     {"本地端口 → 目标服务 · 右键托盘图标可再次打开", "local ports → target services · right-click the tray icon to reopen"},
	"btn.new":      {"＋ 新增", "+ New"},
	"btn.import":   {"导入", "Import"},
	"btn.export":   {"导出", "Export"},
	"btn.log":      {"日志", "Log"},
	"empty.rules":  {"还没有转发规则，点右上角「＋ 新增」加一条", "No rules yet — click \"+ New\" in the top right"},
	"name.blank":   {"未命名", "Untitled"},
	"map.arrow":    {"   →   ", "   →   "},
	"map.local":    {"本机 ", "local "},
	"conn.count":   {"   ·   %d 连接", "   ·   %d conn"},
	"st.stopped":   {"已停止", "Stopped"},
	"st.managed":   {"服务托管", "Service"},
	"st.running":   {"运行中", "Running"},
	"st.failed":    {"失败", "Failed"},
	"btn.start":    {"启动", "Start"},
	"btn.stop":     {"停止", "Stop"},
	"btn.edit":     {"编辑", "Edit"},
	"btn.delete":   {"删除", "Delete"},
	"footer.count": {"共 %d 条 · 运行中 %d 条", "%d rules · %d running"},
	"footer.page":  {"滚轮翻页 %d/%d", "page %d/%d"},

	// 启停失败
	"err.start": {"启动失败：%s\n\n本地端口 %d 起不来。通常是这个端口已被别的程序占用（比如旧的转发任务），\n或者需要管理员权限。",
		"Start failed: %s\n\nLocal port %d could not be opened. Usually the port is already taken\n(an old relay task, another program), or it needs administrator rights."},

	// 导入导出
	"export.title":  {"导出转发配置", "Export rules"},
	"import.title":  {"导入转发配置", "Import rules"},
	"file.filter":   {"JSON 配置 (*.json)\x00*.json\x00所有文件 (*.*)\x00*.*\x00\x00", "JSON config (*.json)\x00*.json\x00All files (*.*)\x00*.*\x00\x00"},
	"err.export":    {"导出失败：", "Export failed: "},
	"ok.export":     {"已导出到\n", "Exported to\n"},
	"err.read":      {"读取失败：", "Read failed: "},
	"err.badconfig": {"不是有效的配置文件：\n", "Not a valid config file:\n"},

	// 新增/编辑对话框
	"dlg.new":    {"新增转发", "New forward"},
	"dlg.edit":   {"编辑转发", "Edit forward"},
	"lbl.name":   {"名称（留空则用 地址:端口）", "Name (blank = host:port)"},
	"lbl.host":   {"目标地址（IP 或域名）", "Target host (IP or domain)"},
	"lbl.tport":  {"目标端口", "Target port"},
	"lbl.lport":  {"本地端口（对外提供服务的端口）", "Local port (served to clients)"},
	"btn.save":   {"保存", "Save"},
	"btn.cancel": {"取消", "Cancel"},
	"hint.save":  {"保存后自动启动", "starts automatically after saving"},
	"err.host":   {"目标地址不能为空", "Target host cannot be empty"},
	"note.title": {"提示", "Notice"},
	"dlg.err.tport": {"目标端口必须是 1-65535", "Target port must be 1-65535"},
	"dlg.err.lport": {"本地端口必须是 1-65535", "Local port must be 1-65535"},
	"dlg.err.inuse": {"本地端口 %d 已被「%s」占用", "Local port %d is already used by \"%s\""},

	// 托盘菜单
	"tray.open":      {"打开面板", "Open panel"},
	"tray.startall":  {"全部启动", "Start all"},
	"tray.stopall":   {"全部停止", "Stop all"},
	"tray.openlog":   {"打开日志", "Open log"},
	"tray.scaleitem": {"界面缩放 %d%%", "UI scale %d%%"},
	"tray.scalemenu": {"界面缩放", "UI scale"},
	"tray.install":   {"安装为后台服务", "Install as service"},
	"tray.uninstall": {"卸载后台服务", "Uninstall service"},
	"tray.exit":      {"退出", "Quit"},
	"tray.lang":      {"语言 / Language", "Language / 语言"},
	"lang.auto":      {"跟随系统", "Follow system"},

	// 服务模式提示
	"svc.install.fail": {"安装服务失败：\n%s\n\n请以管理员身份运行本程序。", "Failed to install the service:\n%s\n\nRun this program as administrator."},
	"svc.install.ok":   {"已安装为开机自启服务，转发交由服务执行。\n配置：%ProgramData%\\PortFwd\\config.json", "Installed as an auto-start service; forwarding now runs there.\nConfig: %ProgramData%\\PortFwd\\config.json"},
	"svc.uninstall.fail": {"卸载服务失败：\n%s", "Failed to remove the service:\n%s"},
	"svc.uninstall.ok":   {"后台服务已卸载，恢复由面板直接转发。", "Service removed; the panel forwards directly again."},
	"svc.install.ok.cli": {"已注册为开机自启服务 PortFwd，并已启动。\n\n配置：%ProgramData%\\PortFwd\\config.json\n面板可以继续用来编辑规则，改完自动生效。",
		"Registered PortFwd as an auto-start service and started it.\n\nConfig: %ProgramData%\\PortFwd\\config.json\nThe panel keeps working for editing; changes apply automatically."},
	"svc.removed.cli": {"服务已停止并删除。", "Service stopped and removed."},

	// 连接记录窗口
	"log.title":       {"连接记录", "Connections"},
	"log.col.time":    {"时间", "Time"},
	"log.col.rule":    {"规则", "Rule"},
	"log.col.src":     {"来源", "Source"},
	"log.col.dst":     {"目标", "Target"},
	"log.col.bytes":   {"流量", "Traffic"},
	"log.col.state":   {"状态", "Status"},
	"log.empty":       {"还没有连接记录。有人访问你转发的端口时会出现在这里。", "No connections yet. They show up here when someone uses one of your forwards."},
	"log.st.failed":   {"失败", "failed"},
	"log.st.open":     {"传输中", "open"},
	"log.st.done":     {"完成 · %s", "done · %s"},
	"log.footer":      {"共 %d 条（滚动查看，最多保留 %d 条）", "%d entries (scroll; keeps the latest %d)"},
	"log.clear":       {"清空", "Clear"},
	"err.unreachable": {"目标不可达: ", "target unreachable: "},
	"err.portinvalid": {"端口无效", "invalid port"},

	// 服务模式错误
	"svc.err.open":     {"打开服务管理器失败（需要管理员权限）: %v", "cannot open the service manager (admin required): %v"},
	"svc.err.create":   {"创建服务失败: %v", "cannot create the service: %v"},
	"svc.err.notfound": {"服务不存在: %v", "service not found: %v"},
	"svc.err.delete":   {"删除服务失败: %v", "cannot delete the service: %v"},
	"svc.display":      {"PortFwd 端口转发", "PortFwd port forwarder"},
}

// systemLangCode 读系统界面语言：中文（含简体/繁体）返回 "zh"，其余 "en"
func systemLangCode() string {
	id, _, _ := kproc.GetUserDefaultUILanguage.Call()
	if id == 0 {
		return "zh"
	}
	if uint32(id)&0x3FF == 0x04 { // LANG_CHINESE
		return "zh"
	}
	return "en"
}

// applyLangPref 按配置里的 lang 设定当前语言；code 为空表示跟随系统
func applyLangPref(code string) {
	switch code {
	case "zh":
		langEN = false
	case "en":
		langEN = true
	default:
		langEN = systemLangCode() != "zh"
	}
}

// T 取界面文案；带参数时按 fmt 格式化
func T(key string, a ...any) string {
	e, ok := dict[key]
	if !ok {
		return key
	}
	v := e.zh
	if langEN {
		v = e.en
	}
	if len(a) > 0 {
		return fmt.Sprintf(v, a...)
	}
	return v
}
