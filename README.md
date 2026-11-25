# LightPan Pro · 轻盘 Pro

基于 [LitePan](https://github.com/Ponphil/LitePan) 的派生项目，提供多网盘管理、STRM 媒体库与 WebDAV / FUSE 挂载。

115 模块使用纯 Go 实现，挂载需要同一账号的 Cookie 和 OpenAPI 凭据，支持分别扫码获取。播放、目录树和同步流程按参考项目移植；真实账号与播放器验收仍待进行。

## 功能

- 多网盘、多账号统一管理，支持文件操作、跨盘传输与离线下载。
- 生成 STRM，对接 Emby / Jellyfin，支持 302 直链与代理播放。
- 媒体整理、刮削及任务自动联动。
- WebDAV 与 FUSE 本地挂载。

具体能力取决于所用网盘和驱动。

## 基础部署

GitHub Actions 构建成功后会发布镜像 `ghcr.io/qiaokes/lightpan-pro`，tag 为 `sha-` 加提交 SHA 的前 7 位，不发布 `latest`。镜像目前支持 `linux/amd64`。

从 Actions 成功记录中取得对应提交后运行（将示例 SHA 替换为实际值）：

```sh
export LIGHTPAN_TAG=sha-abcdef0
docker run -d \
  --name lightpan-pro \
  --restart unless-stopped \
  -p 5211:5211 \
  -e TZ=Asia/Shanghai \
  -v "$PWD/data:/app/data" \
  -v "$PWD/strm:/app/strm" \
  "ghcr.io/qiaokes/lightpan-pro:$LIGHTPAN_TAG"
```

使用仓库中的 Compose 配置时，同样先设置 `LIGHTPAN_TAG`，然后运行 `docker compose up -d`。Compose 包含 FUSE 权限配置，使用前按宿主机环境调整。

也可以从本仓库构建并运行：

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

## CI 与镜像发布

- Push 或手动运行直接构建并发布对应 SHA 的 Docker 镜像。
- 流水线只负责镜像构建和推送，前端资源在 Docker 构建中生成。
- 发布使用仓库的 `GITHUB_TOKEN`，无需额外配置 Docker Hub 账号。

## 来源与许可

感谢 [Ponphil/LitePan](https://github.com/Ponphil/LitePan) 及其贡献者。

沿用上游 [PolyForm Noncommercial 1.0.0](LICENSE) 许可。贡献致谢见 [ACKNOWLEDGEMENTS.md](ACKNOWLEDGEMENTS.md)，第三方声明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
