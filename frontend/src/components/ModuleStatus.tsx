import { useTranslation } from "react-i18next";
import { config, engine } from "../../wailsjs/go/models";

const MODULE_KEYS = ["detection", "analysis", "fpv", "jamming"] as const;
type EngineAction = "starting" | "stopping" | null;

interface Props {
  cfg: config.Config | null;
  status: engine.Status | null;
  onStart: () => void;
  onStop: () => void;
  actionState: EngineAction;
}

export default function ModuleStatus({
  cfg,
  status,
  onStart,
  onStop,
  actionState,
}: Props) {
  const { t, i18n } = useTranslation();
  const running = status?.running ?? false;
  const pending = actionState !== null;
  const badgeClass =
    actionState === "starting" || actionState === "stopping"
      ? "badge-pending"
      : running
        ? "badge-running"
        : "badge-stopped";
  const badgeLabel =
    actionState === "starting"
      ? t("app.starting")
      : actionState === "stopping"
        ? t("app.stopping")
        : running
          ? t("app.running")
          : t("app.stopped");
  const buttonClass =
    actionState === "stopping" || (running && actionState !== "starting")
      ? "btn-danger"
      : "btn-primary";
  const buttonLabel =
    actionState === "starting"
      ? t("app.starting")
      : actionState === "stopping"
        ? t("app.stopping")
        : running
          ? t("app.stop")
          : t("app.start");

  const getModule = (name: string) =>
    status?.modules?.find((m) => m.name === name);

  const isModuleEnabled = (name: (typeof MODULE_KEYS)[number]) => {
    switch (name) {
      case "detection":
        return cfg?.detections?.length
          ? cfg.detections.some((detection) => detection.enabled !== false)
          : cfg?.detection?.enabled !== false;
      case "analysis":
        return cfg?.analysis?.enabled !== false;
      case "fpv":
        return cfg?.fpv?.enabled !== false;
      case "jamming":
        return cfg?.jamming?.enabled !== false;
    }
  };

  const enabledModuleCount = MODULE_KEYS.filter((key) => isModuleEnabled(key))
    .length;
  const totalConnections =
    status?.modules?.reduce(
      (count, module) => count + (module.connectionCount ?? 0),
      0,
    ) ?? 0;
  const formatLastActivity = (value?: string) => {
    if (!value) {
      return t("status.none");
    }

    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return value;
    }

    return new Intl.DateTimeFormat(
      i18n.language === "zh" ? "zh-CN" : "en-US",
      {
        hour12: false,
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      },
    ).format(date);
  };

  return (
    <div className="module-status">
      <div className="status-header">
        <div className="status-header-left">
          <span className="panel-title">{t("status.title")}</span>
          <div className="status-summary">
            <span className="status-summary-item">
              {t("status.droneCount", { count: status?.droneCount ?? 0 })}
            </span>
            <span className="status-summary-item">
              {t("status.activeConnections", { count: totalConnections })}
            </span>
            <span className="status-summary-item">
              {t("status.enabledModules", { count: enabledModuleCount })}
            </span>
          </div>
        </div>
        <div className="engine-controls">
          <span className={`engine-badge ${badgeClass}`}>{badgeLabel}</span>
          <button
            className={`btn engine-action-btn ${buttonClass}${pending ? " is-pending" : ""}`}
            onClick={running ? onStop : onStart}
            disabled={pending}
          >
            <span className="engine-action-content">
              <span className="engine-action-indicator" />
              <span>{buttonLabel}</span>
            </span>
          </button>
        </div>
      </div>
      <div className="module-cards">
        {MODULE_KEYS.map((key) => {
          const mod = getModule(key);
          const enabled = isModuleEnabled(key);
          const connected = enabled && (mod?.connected ?? false);
          const connecting = enabled && actionState === "starting" && !connected;
          const detectionDebug =
            key === "detection" && enabled ? (
              <div className="module-debug">
                <div className="module-debug-line">
                  <span className="module-debug-label">
                    {t("status.lastPacket")}
                  </span>
                  <span>{formatLastActivity(mod?.lastActivityAt)}</span>
                </div>
                <div className="module-debug-line">
                  <span className="module-debug-label">
                    {t("status.activeClientsLabel")}
                  </span>
                  <span className="module-debug-value">
                    {mod?.clientAddresses?.length
                      ? mod.clientAddresses.join(", ")
                      : t("status.none")}
                  </span>
                </div>
              </div>
            ) : null;
          const cardClass = !enabled
            ? "card-disabled"
            : connected
              ? "card-connected"
              : connecting
                ? "card-connecting"
                : "";
          return (
            <div key={key} className={`module-card ${cardClass}`}>
              <div className="module-card-top">
                <div className="module-dot" />
                <div className="module-name">{t(`status.${key}`)}</div>
              </div>
              <div className="module-state">
                {!enabled
                  ? t("status.disabled")
                  : connecting
                    ? t("status.connecting")
                    : connected
                      ? t("status.connected")
                    : t("status.disconnected")}
              </div>
              <div className="module-meta">
                {t("status.connections", {
                  count: mod?.connectionCount ?? 0,
                })}
              </div>
              {detectionDebug}
            </div>
          );
        })}
      </div>
    </div>
  );
}
