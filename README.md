# README

精准测向设备的启用、连接和模拟范围见 [精准测向模拟说明](docs/precision-direction-finding.md)。

多面普通侦测模块的跳频、实时 RSSI、可拖拽雷达图与缺面测试见 [联合测向模拟说明](docs/joint-direction.md)。

## About

This is the official Wails React-TS template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

## 界面语言

界面控件与 `uav_defender_util` 统一使用 Ant Design，包括下拉选择、输入框、数字输入、开关、复选框和确认弹窗；文件导入、导出继续使用系统文件选择器。

支持中文、英语、土耳其语和法语，可在右上角选择并自动记住。场景导入、导出、覆盖及退出保存弹窗跟随界面语言；原始协议日志和错误技术详情保留原文。

如需限定发行版语言，可设置 `VITE_APP_LANGUAGES=zh,en,tr,fr`，例如 `VITE_APP_LANGUAGES=fr wails build` 构建法语版。语言代码使用 `zh`、`en`、`tr`、`fr`。前端翻译完整性与语言切换测试可在 `frontend/` 内运行 `bun test`。
