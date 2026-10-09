export const minListenPort = __MIN_LISTEN_PORT__;
export const listenPortHelp = __STANDALONE__
  ? "范围 1–65535；请同时配置 Docker 端口映射，并避开管理端口和已占用的端口。"
  : minListenPort === 1
  ? "范围 1–65535；低位端口由内置 Nginx 的专用绑定权限支持，端口不能被其他服务占用。"
  : "标准版仅允许监听 1024–65535；需要直接监听 80/443 时，请安装同版本的全端口版。";
