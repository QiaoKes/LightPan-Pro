# LightPan Pro · 轻盘 Pro

基于 [LitePan](https://github.com/Ponphil/LitePan) 的派生项目，提供多网盘管理、STRM 媒体库与 WebDAV / FUSE 挂载。

目前沿用上游 Go 版功能；115 直链播放优化仍在方案阶段。

## 功能

- 多网盘、多账号统一管理，支持文件操作、跨盘传输与离线下载。
- 生成 STRM，对接 Emby / Jellyfin，支持 302 直链与代理播放。
- 媒体整理、刮削及任务自动联动。
- WebDAV 与 FUSE 本地挂载。

具体能力取决于所用网盘和驱动。

## 基础部署

使用 Docker 从本仓库构建并运行：

```sh
git clone https://github.com/QiaoKes/LightPan-Pro.git
cd LightPan-Pro
docker build -t lightpan-pro:local .
docker run -d \
  --name lightpan-pro \
  --restart unless-stopped \
  -p 5211:5211 \
  -e TZ=Asia/Shanghai \
  -v "$PWD/data:/app/data" \
  -v "$PWD/strm:/app/strm" \
  lightpan-pro:local
```

访问 `http://服务器IP:5211`，初始用户名和密码均为 `admin`，首次登录后修改。

上述配置用于基础运行。FUSE 挂载需额外配置 `/dev/fuse`、挂载目录与相关权限；磁力下载需按配置映射 TCP / UDP 监听端口。

## 来源与许可

感谢 [Ponphil/LitePan](https://github.com/Ponphil/LitePan) 及其贡献者。

沿用上游 [PolyForm Noncommercial 1.0.0](LICENSE) 许可。贡献致谢见 [ACKNOWLEDGEMENTS.md](ACKNOWLEDGEMENTS.md)，第三方声明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
