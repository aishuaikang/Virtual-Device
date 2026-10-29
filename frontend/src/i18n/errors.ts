export function errorText(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function errorReason(error: unknown): string | undefined {
  const message = errorText(error);
  if (/address already in use|only one usage|端口.*占用/i.test(message)) return "errors.addressInUse";
  if (/permission denied|access is denied|权限不足/i.test(message)) return "errors.permission";
  if (/timeout|timed out|deadline exceeded|超时/i.test(message)) return "errors.timeout";
  if (/connection refused|network is unreachable|no route to host|连接.*失败/i.test(message)) return "errors.connection";
  if (/运行中|请先停止模拟器/.test(message)) return "errors.runningConfig";
  if (/配置.*(错误|无效|失败)|必须|不能为|不能为空|invalid config/i.test(message)) return "errors.invalidConfig";
  return undefined;
}
