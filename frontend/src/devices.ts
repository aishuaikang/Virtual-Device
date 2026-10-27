import { config, engine } from "../wailsjs/go/models";

export const DEVICES = [
  { id: "detection" },
  { id: "analysis" },
  { id: "precision_df" },
  { id: "directed_strike" },
  { id: "fpv" },
  { id: "jamming" },
] as const;

export type DeviceId = (typeof DEVICES)[number]["id"];
export type PageId = DeviceId | "targets";

export function detections(cfg: config.Config) {
  return cfg.detections?.length ? cfg.detections : [cfg.detection];
}

export function deviceEnabled(cfg: config.Config, id: DeviceId) {
  return id === "detection"
    ? detections(cfg).some((item) => item.enabled)
    : cfg[id].enabled;
}

export function toggleDevice(
  cfg: config.Config,
  id: DeviceId,
  enabled: boolean,
) {
  if (id === "detection") {
    const next = detections(cfg).map((item) => ({ ...item, enabled }));
    return new config.Config({ ...cfg, detections: next, detection: next[0] });
  }
  return new config.Config({ ...cfg, [id]: { ...cfg[id], enabled } });
}

export function deviceEndpoint(cfg: config.Config, id: DeviceId) {
  if (id === "detection") {
    const items = detections(cfg);
    return items.map((item) => `${item.host}:${item.port}`).join(", ");
  }
  if (id === "precision_df") {
    return `${cfg.precision_df.host}:${cfg.precision_df.command_port}`;
  }
  const item = cfg[id];
  if ("hosts" in item) {
    return (item.hosts ?? []).map((host) => `${host}:${item.port}`).join(", ");
  }
  return `${item.host}:${item.port}`;
}

export function deviceState(
  cfg: config.Config,
  id: DeviceId,
  status: engine.Status | null,
) {
  if (!deviceEnabled(cfg, id)) return "disabled";
  if (!status?.running) return "stopped";
  const module = status.modules?.find((item) => item.name === id);
  return module?.connected ? "connected" : "waiting";
}
