const commandKeys: Record<string, string> = {
  "心跳": "heartbeat", "功放状态查询": "ampStatus", "功放 ATT 读写": "attenuation",
  "信号源使能": "signalSource", "宽带频率配置": "wideband", "窄带功放控制": "narrowband",
  "云台停止": "ptzStop", "保存云台预置位": "ptzPreset", "云台运动": "ptzMotion",
  "云台水平定位": "horizontalLocate", "云台俯仰定位": "pitchLocate",
  "云台水平角查询": "horizontalQuery", "云台俯仰角查询": "pitchQuery",
  "云台速度设置": "speedSet", "云台速度查询": "speedQuery",
};

export function localizeCommand(command: string, translate: (key: string) => string): string {
  const key = Object.hasOwn(commandKeys, command) ? commandKeys[command] : undefined;
  return key ? translate(`commands.${key}`) : command;
}
