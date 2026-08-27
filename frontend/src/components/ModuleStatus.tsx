import { useTranslation } from "react-i18next";
import { config, engine } from "../../wailsjs/go/models";

const MODULES = [
  { key: "detection", statusName: "detection" },
  { key: "analysis", statusName: "analysis" },
  { key: "fpv", statusName: "fpv" },
  { key: "jamming", statusName: "jamming" },
  { key: "directedStrike", statusName: "directed_strike" },
] as const;

type EngineAction = "starting" | "stopping" | null;

interface Props {
  cfg: config.Config | null;
  status: engine.Status | null;
  actionState: EngineAction;
}

export default function ModuleStatus({ cfg, status, actionState }: Props) {
  const { t } = useTranslation();
  const running = status?.running ?? false;
  const pending = actionState !== null;
  const getModule = (name: string) =>
    status?.modules?.find((module) => module.name === name);

  const isModuleEnabled = (key: (typeof MODULES)[number]["key"]) => {
    switch (key) {
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
      case "directedStrike":
        return cfg?.directed_strike?.enabled !== false;
    }
  };

  const enabledModules = MODULES.filter(({ key }) => isModuleEnabled(key));
  const totalConnections =
    status?.modules?.reduce(
      (total, module) => total + (module.connectionCount ?? 0),
      0,
    ) ?? 0;
  const badgeClass = pending
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

  return (
    <section className="module-status">
      <div className="status-header">
        <div className="status-header-left">
          <span className="panel-eyebrow">SYSTEM OVERVIEW</span>
          <span className="panel-title">{t("status.title")}</span>
          <div className="status-summary">
            <span>{t("status.droneCount", { count: status?.droneCount ?? 0 })}</span>
            <span>{t("status.activeConnections", { count: totalConnections })}</span>
            <span>{t("status.enabledModules", { count: enabledModules.length })}</span>
          </div>
        </div>
        <span className={`engine-badge ${badgeClass}`}>
          <span />
          {badgeLabel}
        </span>
      </div>

      <div className="module-cards">
        {MODULES.map(({ key, statusName }) => {
          const module = getModule(statusName);
          const enabled = isModuleEnabled(key);
          const connected = enabled && (module?.connected ?? false);
          const directedListening =
            key === "directedStrike" &&
            running &&
            Boolean(status?.directedStrike?.listening);
          const connecting =
            enabled &&
            !connected &&
            (actionState === "starting" || directedListening);
          const cardClass = !enabled
            ? "card-disabled"
            : connected
              ? "card-connected"
              : connecting
                ? "card-connecting"
                : "";
          const stateLabel = !enabled
            ? t("status.disabled")
            : connected
              ? t("status.connected")
              : connecting
                ? key === "directedStrike"
                  ? t("status.listening")
                  : t("status.connecting")
                : t("status.disconnected");

          return (
            <article key={key} className={`module-card ${cardClass}`}>
              <div className="module-card-top">
                <span className="module-dot" />
                <span className="module-name">{t(`status.${key}`)}</span>
              </div>
              <strong className="module-state">{stateLabel}</strong>
              <span className="module-meta">
                {t("status.connections", {
                  count: module?.connectionCount ?? 0,
                })}
              </span>
            </article>
          );
        })}
      </div>
    </section>
  );
}
