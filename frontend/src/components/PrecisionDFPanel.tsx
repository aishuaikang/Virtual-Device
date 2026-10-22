import { useTranslation } from "react-i18next";
import { config, modules } from "../../wailsjs/go/models";

interface Props {
  cfg: config.PrecisionDFConfig;
  snapshot: modules.PrecisionDFSnapshot | undefined;
  running: boolean;
}

export default function PrecisionDFPanel({ cfg, snapshot, running }: Props) {
  const { t } = useTranslation();
  const connected = running && snapshot?.commandConnected && snapshot?.dataConnected;
  const state = !running ? "idle" : connected ? "connected" : "listening";
  const endpoint = snapshot?.commandAddress ?? `${cfg.host}:${cfg.command_port}`;

  return (
    <section className={`directed-strike-panel precision-df-panel is-${state}`}>
      <div className="operations-heading">
        <div>
          <span className="operations-kicker">PRECISION DF · TCP</span>
          <h2>{t("precisionDF.title")}</h2>
        </div>
        <span className="operations-state">
          <span className="operations-state-dot" />
          {t(`directedStrike.${state}`)}
        </span>
      </div>
      <div className="endpoint-card">
        <div className="endpoint-copy">
          <span>{t("precisionDF.commandEndpoint")}</span>
          <strong>{endpoint}</strong>
          <small>{t("precisionDF.connectHint")}</small>
        </div>
      </div>
      <div className="precision-df-links">
        <span>{t("precisionDF.commandLink")}</span>
        <strong>{t(snapshot?.commandConnected && running ? "status.connected" : "status.disconnected")}</strong>
        <span>{t("precisionDF.dataLink")}</span>
        <strong>{t(snapshot?.dataConnected && running ? "status.connected" : "status.disconnected")}</strong>
        <span>{t("precisionDF.dataEndpoint")}</span>
        <code>{snapshot?.dataAddress ?? `${cfg.host}:${cfg.data_port}`}</code>
      </div>
      <div className="telemetry-grid">
        <div className="telemetry-cell">
          <span>RX</span><strong>{snapshot?.receivedCommands ?? 0}</strong>
          <small>{t("precisionDF.commands")}</small>
        </div>
        <div className="telemetry-cell">
          <span>TX</span><strong>{snapshot?.sentFrames ?? 0}</strong>
          <small>{t("directedStrike.frames")}</small>
        </div>
        <div className="telemetry-cell telemetry-cell-accent">
          <span>IQ</span><strong>{((snapshot?.iqBytes ?? 0) / 1048576).toFixed(2)}</strong>
          <small>MiB</small>
        </div>
      </div>
      <section className="operation-section">
        <div className="operation-section-title">
          <span>{t("precisionDF.task")}</span>
          <small>{running && snapshot?.running ? t("app.running") : t("directedStrike.standby")}</small>
        </div>
        <div className="precision-df-links">
          <span>{t("precisionDF.mode")}</span><strong>{snapshot?.mode || "—"}</strong>
          <span>TASK</span><strong>{snapshot?.taskID || "—"}</strong>
          <span>{t("precisionDF.bearingValue")}</span><strong>{cfg.bearing_deg === -1 ? t("precisionDF.invalid") : `${cfg.bearing_deg}°`}</strong>
        </div>
      </section>
      <section className="operation-section latest-command-section">
        <div className="operation-section-title"><span>{t("directedStrike.latestCommand")}</span></div>
        <code className="precision-df-command">{snapshot?.lastCommand || t("overview.noInteraction")}</code>
        {snapshot?.lastError && <p className="operation-error">{snapshot.lastError}</p>}
      </section>
      <p className="operation-empty">{t("precisionDF.simulationHint")}</p>
    </section>
  );
}
