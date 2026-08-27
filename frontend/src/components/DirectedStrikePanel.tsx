import type { CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { config, modules } from "../../wailsjs/go/models";

interface Props {
  config: config.DirectedStrikeConfig | undefined;
  snapshot: modules.DirectedStrikeSnapshot | undefined;
  running: boolean;
}

const SOURCE_BANDS: Record<string, string> = {
  "0x02": "1–2 GHz",
  "0x03": "2–4 GHz",
  "0x04": "4–6 GHz",
  "0x05": "6–8 GHz",
};

function formatActivity(value: string | undefined, locale: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale === "zh" ? "zh-CN" : "en-US", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

export default function DirectedStrikePanel({
  config: simulatorConfig,
  snapshot,
  running,
}: Props) {
  const { t, i18n } = useTranslation();
  const enabled = simulatorConfig?.enabled !== false;
  const connected = running && (snapshot?.activeConnections ?? 0) > 0;
  const listening = running && Boolean(snapshot?.listening);
  const endpoint =
    snapshot?.listenAddress ??
    `${simulatorConfig?.host || "0.0.0.0"}:${simulatorConfig?.port || 19000}`;
  const stateClass = !enabled
    ? "is-disabled"
    : connected
      ? "is-connected"
      : listening
        ? "is-listening"
        : "is-idle";
  const stateLabel = !enabled
    ? t("directedStrike.disabled")
    : connected
      ? t("directedStrike.connected")
      : listening
        ? t("directedStrike.listening")
        : t("directedStrike.idle");
  const frequencies = new Map(
    (snapshot?.widebandConfigs ?? []).map((item) => [item.address, item]),
  );
  const ptz = snapshot?.ptz;
  const ptzStyle = {
    "--ptz-angle": `${ptz?.horizontalAngle ?? 0}deg`,
  } as CSSProperties;

  return (
    <div className={`directed-strike-panel ${stateClass}`}>
      <div className="operations-heading">
        <div>
          <span className="operations-kicker">DIRECTED STRIKE</span>
          <h2>{t("directedStrike.title")}</h2>
        </div>
        <span className="operations-state">
          <span className="operations-state-dot" />
          {stateLabel}
        </span>
      </div>

      <section className="endpoint-card" aria-label={t("directedStrike.endpoint")}>
        <div className="endpoint-radar" aria-hidden="true">
          <span className="endpoint-radar-ring" />
          <span className="endpoint-radar-core" />
        </div>
        <div className="endpoint-copy">
          <span>{t("directedStrike.endpoint")}</span>
          <strong>{endpoint}</strong>
          <small>
            {connected
              ? t("directedStrike.clients", {
                  count: snapshot?.activeConnections ?? 0,
                })
              : listening
                ? t("directedStrike.waitingClient")
                : t("directedStrike.startHint")}
          </small>
        </div>
      </section>

      <div className="telemetry-grid">
        <div className="telemetry-cell">
          <span>RX</span>
          <strong>{snapshot?.receivedFrames ?? 0}</strong>
          <small>{t("directedStrike.frames")}</small>
        </div>
        <div className="telemetry-cell">
          <span>TX</span>
          <strong>{snapshot?.sentFrames ?? 0}</strong>
          <small>{t("directedStrike.frames")}</small>
        </div>
        <div className="telemetry-cell telemetry-cell-accent">
          <span>HB</span>
          <strong>{snapshot?.heartbeatCount ?? 0}</strong>
          <small>{t("directedStrike.heartbeats")}</small>
        </div>
      </div>

      <section className="operation-section latest-command-section">
        <div className="operation-section-title">
          <span>{t("directedStrike.latestCommand")}</span>
          <time>
            {formatActivity(snapshot?.lastActivityAt, i18n.language)}
          </time>
        </div>
        {snapshot?.lastCommand ? (
          <div className="latest-command">
            <span className="command-pulse" />
            <div>
              <strong>{snapshot.lastCommand}</strong>
              <small>{snapshot.lastCommandDetail || t("directedStrike.acknowledged")}</small>
            </div>
          </div>
        ) : (
          <p className="operation-empty">{t("directedStrike.noCommand")}</p>
        )}
        {snapshot?.lastError && (
          <p className="operation-error">{snapshot.lastError}</p>
        )}
      </section>

      <section className="operation-section">
        <div className="operation-section-title">
          <span>{t("directedStrike.widebandSources")}</span>
          <small>
            {(snapshot?.signalSources ?? []).filter((source) => source.enabled)
              .length}
            /4 {t("directedStrike.active")}
          </small>
        </div>
        <div className="source-stack">
          {(snapshot?.signalSources ?? [
            { address: "0x02", enabled: false },
            { address: "0x03", enabled: false },
            { address: "0x04", enabled: false },
            { address: "0x05", enabled: false },
          ]).map((source) => {
            const frequency = frequencies.get(source.address);
            return (
              <div
                key={source.address}
                className={`source-row${source.enabled ? " is-active" : ""}`}
              >
                <span className="source-address">{source.address}</span>
                <span className="source-band">{SOURCE_BANDS[source.address]}</span>
                <span className="source-frequency">
                  {frequency
                    ? `${frequency.startFreqMHz}–${frequency.endFreqMHz} MHz`
                    : t("directedStrike.unconfigured")}
                </span>
                <span className="source-switch" aria-label={source.enabled ? "on" : "off"} />
              </div>
            );
          })}
        </div>
      </section>

      <section className="operation-section ptz-section">
        <div className="operation-section-title">
          <span>{t("directedStrike.ptz")}</span>
          <small>{ptz?.action || t("directedStrike.standby")}</small>
        </div>
        <div className="ptz-readout">
          <div className="ptz-dial" style={ptzStyle} aria-hidden="true">
            <span className="ptz-axis" />
            <span className="ptz-center" />
          </div>
          <div className="ptz-values">
            <div>
              <span>H</span>
              <strong>{(ptz?.horizontalAngle ?? 0).toFixed(2)}°</strong>
            </div>
            <div>
              <span>V</span>
              <strong>{(ptz?.pitchAngle ?? 0).toFixed(2)}°</strong>
            </div>
            <small>
              {t("directedStrike.locateSpeed", {
                horizontal: ptz?.locateHorizontalSpeed ?? 32,
                vertical: ptz?.locateVerticalSpeed ?? 34,
              })}
            </small>
          </div>
        </div>
      </section>

      <section className="operation-section">
        <div className="operation-section-title">
          <span>{t("directedStrike.narrowbandAmps")}</span>
        </div>
        <div className="amp-grid">
          {(snapshot?.narrowbandAmps ?? [
            { address: "0x01", enabled: false },
            { address: "0x02", enabled: false },
            { address: "0x03", enabled: false },
            { address: "0x04", enabled: false },
            { address: "0x05", enabled: false },
          ]).map((amp) => (
            <span
              key={amp.address}
              className={`amp-chip${amp.enabled ? " is-active" : ""}`}
            >
              <i /> {amp.address}
            </span>
          ))}
        </div>
      </section>

      {snapshot?.clientAddresses?.length ? (
        <p className="client-addresses">
          {t("directedStrike.clientAddress")}: {snapshot.clientAddresses.join(", ")}
        </p>
      ) : null}
    </div>
  );
}
