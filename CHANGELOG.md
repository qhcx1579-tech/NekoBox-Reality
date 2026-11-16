# Changelog / 更新记录

## 1.4.2-reality-11

- Added manual proxy exit IP and country lookup through IPPure HTTPS. Results are not saved, and failed queries do not fall back to a direct connection.
- Fixed hidden group tabs when only one custom group is visible, and corrected group selection after deletion.
- Fixed missing traffic speed updates on a fresh installation by aligning the default refresh interval with the settings page.
- Removed full node links from import debug logs and restricted parser failure messages to avoid exposing credentials.

- 新增通过 IPPure HTTPS 手动查询代理出口 IP 和国家/地区。结果不保存，查询失败不回退直连。
- 修复仅有一个自建分组时主页分组栏隐藏，以及删除分组后的选择回退问题。
- 统一首次安装时的流量刷新默认间隔，修复速率不显示的问题。
- 移除导入调试日志中的完整节点链接，并限制解析失败日志内容，避免输出连接凭据。
