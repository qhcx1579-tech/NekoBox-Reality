# NekoBox Reality for Android

An Android proxy client with VLESS XHTTP and REALITY support.

支持 VLESS XHTTP 与 REALITY 的 Android 代理客户端。

## Features / 功能

- VLESS with TCP + REALITY and XHTTP.
- XHTTP modes: `auto`, `packet-up`, `stream-up`, and `stream-one`.
- Subscription import, routing rules, per-app proxy settings, and traffic statistics.
- ARM64 builds with 16 KB native library alignment.
- On-demand proxy exit IP and country lookup using IPPure over HTTPS; query results are not saved.

- 支持 VLESS TCP + REALITY 和 XHTTP。
- 支持订阅导入、分流规则、分应用代理和流量统计。
- 提供 ARM64 安装包，原生库支持 16 KB 对齐。
- 支持通过 IPPure HTTPS 手动查询代理出口 IP 和国家/地区，查询结果不保存。

## Downloads / 下载

[GitHub Releases](https://github.com/qhcx1579-tech/NekoBox-Reality/releases)

## Getting started / 使用

1. Install the APK for your device architecture.
2. Import a subscription or node link.
3. Check that the transport and security settings match the server, then connect.

1. 安装适合设备架构的 APK。
2. 导入订阅或节点链接。
3. 确认传输协议和安全设置与服务端一致，再启动连接。

For XHTTP, keep Host, path, mode, and any custom parameters consistent with the server.

使用 XHTTP 时，Host、路径、模式及自定义参数需与服务端一致。

After connecting, tap “View proxy exit IP” on the home screen to query the selected proxy. IPPure sees the query connection’s exit IP. Other traffic may follow different routing rules.

连接后，点击主页“查看代理出口 IP”即可查询所选代理。IPPure 会看到查询连接的出口 IP；其他流量可能遵循不同分流规则。

## XHTTP limitations / XHTTP 支持范围

XHTTP supports VLESS with plain transport, TLS, or REALITY. HTTP/3, separate `downloadSettings`, ECH on the XHTTP path, and VLESS Encryption other than `none` are currently unsupported. Disable ordinary Multiplex for XHTTP nodes.

XHTTP 支持 VLESS 搭配普通传输、TLS 或 REALITY。暂不支持 HTTP/3、独立 `downloadSettings`、XHTTP 路径的 ECH，以及非 `none` 的 VLESS Encryption。XHTTP 节点请关闭普通 Multiplex。

## REALITY compatibility / REALITY 兼容性

TCP + REALITY uses sing-box and declares REALITY handshake version `1.8.1`. A server that requires a higher minimum client version may reject the connection. The app version and the REALITY handshake version are separate. XHTTP uses the bundled Xray core.

TCP + REALITY 使用 sing-box，其 REALITY 握手声明版本为 `1.8.1`。服务端若要求更高的最低客户端版本，可能拒绝连接。应用版本与 REALITY 握手版本是不同的编号；XHTTP 使用内置 Xray 内核。

## Building / 构建

[Build requirements and instructions / 构建环境与步骤](BUILDING.md)

## License and acknowledgements / 许可与致谢

This is an independent derivative of [NekoBox for Android](https://github.com/MatsuriDayo/NekoBoxForAndroid), distributed under the project's [GPL-3.0 license](LICENSE). Original copyright and author information are retained.

本项目基于 NekoBox for Android 独立开发，保留原项目的版权、作者信息及 GPL-3.0 许可证。

- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- [XTLS/Xray-core](https://github.com/XTLS/Xray-core) · [MPL-2.0 license](xray-helper/LICENSE.xray)
- [shadowsocks/shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android)
- [SagerNet/SagerNet](https://github.com/SagerNet/SagerNet)
- [Yacd-meta](https://github.com/MetaCubeX/Yacd-meta)
