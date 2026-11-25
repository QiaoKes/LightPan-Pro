# 第三方软件声明

LitePan 使用以下第三方组件及协议参考。各组件仍由其原作者持有版权，并遵循各自许可证。

## p115client / p115cipher / p115pickcode

- 用途：115 请求协议、加密固定向量及 pickcode 转换的纯 Go 移植参考；运行时不加载 Python。
- 作者：ChenyangGao。
- 来源：本地固定版本 p115client `b649be77a6c77681e26d6eb1d1d013418b49d5e8` 及 EmbyNginx 实际内置 SDK/cipher；具体文件、哈希、版本差异见 `.trellis/tasks/11-25-115-dual-auth-go/research/protocol-baseline.md`。
- p115cipher 内置版本为 0.0.5。上游源码声明 GPLv3，而包元数据声明 MIT，存在未解决的声明差异。本项目保留原始 GPLv3 许可证及差异说明，不擅自将其改标为 MIT。
- 原始许可证：`drivers/115/client/cipher/LICENSE.upstream`；来源说明：`drivers/115/client/cipher/NOTICE`。对外分发前需解决适用授权与项目许可证兼容性问题。

## heic-to / libheif

- 用途：在浏览器不原生支持时解码 HEIC / HEIF 图片。
- 项目：https://github.com/hoppergee/heic-to
- 许可证：GNU Lesser General Public License v3.0 or later（LGPL-3.0-or-later）
- 许可证副本：前端静态资源 `/licenses/heic-to-LGPL-3.0.txt`

该组件作为独立的按需加载模块分发，LitePan 未修改其源码。

## libbitsub

- 用途：在浏览器内解析并渲染外置 Blu-ray PGS / SUP 位图字幕。
- 项目：https://github.com/altqx/libbitsub
- 许可证：MIT License
- 许可证副本：前端静态资源 `/licenses/libbitsub-MIT.txt`

该组件使用 WebAssembly，并且仅在用户实际选择 SUP 字幕时按需加载；LitePan 未修改其源码。

## docx-preview / docxjs

- 用途：在浏览器内解析并分页渲染 DOCX 文档。
- 项目：https://github.com/VolodymyrBaydalka/docxjs
- 许可证：Apache License 2.0

该组件仅在用户打开 DOCX 文档时按需加载；LitePan 未修改其源码。

## SheetJS Community Edition

- 用途：在浏览器内解析 XLSX、XLS、CSV 和 ODS 表格文件。
- 项目：https://git.sheetjs.com/SheetJS/sheetjs
- 许可证：Apache License 2.0

该组件仅在用户打开表格文件时按需加载；LitePan 未修改其源码。

## zip.js

- 用途：在浏览器内分段读取 ZIP / CBZ 压缩包的目录信息。
- 项目：https://github.com/gildas-lormeau/zip.js
- 许可证：BSD-3-Clause

该组件仅在用户打开 ZIP / CBZ 文件时按需加载；LitePan 未修改其源码。

## pptx-renderer

- 用途：在浏览器内解析并渲染 PPTX 演示文稿。
- 项目：https://github.com/aiden0z/pptx-renderer
- 许可证：Apache License 2.0

该组件仅在用户打开 PPTX 文件时按需加载，不向第三方服务上传文档；LitePan 未修改其源码。

## charset-normalizer / CPython 字符集行为

- 用途：115 目录树文件的本地字符集探测、解码与 Python 行为对照；运行和构建不加载 Python。
- 探测算法：charset-normalizer 3.5.1，MIT License，Copyright (c) 2025 TAHRI Ahmed R.；固定源码 commit `e239bdc5cc1eb1f0db08d4046ad531f805dbea71`。
- 编码映射、Unicode 属性及排序/数值行为参考：CPython 3.12.13，固定源码 commit `3bb231a6a5dc02b95658877318bf61501a7209e9`；ISO-2022 源文件署名 Hye-Shik Chang。
- 原始声明与许可证：`drivers/115/client/charset/NOTICE`、`LICENSE.upstream`、`LICENSE.cpython`。
- 移植及离线生成对照证据：`.trellis/tasks/11-25-115-dual-auth-go/research/charset-oracle.md`。
