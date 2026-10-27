import { useTranslation } from "react-i18next";
import { engine } from "../../wailsjs/go/models";
import type { DeviceId } from "../devices";

export default function DeviceTelemetry({
  device,
  status,
}: {
  device: DeviceId;
  status: engine.Status | null;
}) {
  const { t } = useTranslation();
  if (!status?.running) return null;
  const module = status.modules?.find((item) => item.name === device);
  const receiver = device === "precision_df" ? status.precisionDF : undefined;
  const directed =
    device === "directed_strike" ? status.directedStrike : undefined;
  const metrics = receiver
    ? [
        ["RX", receiver.receivedCommands],
        ["TX", receiver.sentFrames],
        ["IQ", `${(receiver.iqBytes / 1048576).toFixed(2)} MiB`],
      ]
    : directed
      ? [
          ["RX", directed.receivedFrames],
          ["TX", directed.sentFrames],
          [t("runtime.heartbeats"), directed.heartbeatCount],
        ]
      : [[t("runtime.connections"), module?.connectionCount ?? 0]];
  const lastCommand = receiver?.lastCommand ?? directed?.lastCommand;
  const lastError = receiver?.lastError ?? directed?.lastError;
  const clients = module?.clientAddresses ?? [];

  return (
    <section className="telemetry" aria-label={t("runtime.title")}>
      <div className="section-heading">
        <h2>{t("runtime.title")}</h2>
        {receiver?.running && (
          <code>
            {receiver.mode} · TASK {receiver.taskID}
          </code>
        )}
      </div>
      <dl className="metrics">
        {metrics.map(([label, value]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      {receiver && (
        <div className="receiver-links">
          <span>
            <i
              className={`state-dot state-${receiver.commandConnected ? "connected" : "stopped"}`}
            />
            {t("runtime.commandLink")}
            <code>{receiver.commandAddress}</code>
          </span>
          <span>
            <i
              className={`state-dot state-${receiver.dataConnected ? "connected" : "stopped"}`}
            />
            {t("runtime.dataLink")}
            <code>{receiver.dataAddress}</code>
          </span>
        </div>
      )}
      {directed && (
        <>
          <div className="telemetry-columns">
            <div>
              <h3>{t("runtime.wideband")}</h3>
              <div className="source-list">
                {directed.signalSources?.map((source) => {
                  const frequency = directed.widebandConfigs?.find(
                    (item) => item.address === source.address,
                  );
                  return (
                    <div className="source-row" key={source.address}>
                      <i
                        className={`state-dot state-${source.enabled ? "connected" : "disabled"}`}
                      />
                      <code>{source.address}</code>
                      <span>
                        {frequency
                          ? `${frequency.startFreqMHz}–${frequency.endFreqMHz} MHz`
                          : "—"}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
            <div>
              <h3>{t("runtime.ptz")}</h3>
              <dl className="ptz-values">
                <div>
                  <dt>H</dt>
                  <dd>{(directed.ptz?.horizontalAngle ?? 0).toFixed(2)}°</dd>
                </div>
                <div>
                  <dt>V</dt>
                  <dd>{(directed.ptz?.pitchAngle ?? 0).toFixed(2)}°</dd>
                </div>
              </dl>
              <h3>{t("runtime.narrowband")}</h3>
              <div className="amp-list">
                {directed.narrowbandAmps?.map((amp) => (
                  <span key={amp.address}>
                    <i
                      className={`state-dot state-${amp.enabled ? "connected" : "disabled"}`}
                    />
                    <code>{amp.address}</code>
                  </span>
                ))}
              </div>
            </div>
          </div>
        </>
      )}
      {lastCommand && (
        <div className="last-command">
          <span>{t("runtime.lastCommand")}</span>
          <code>{lastCommand}</code>
          {directed?.lastCommandDetail && (
            <small>{directed.lastCommandDetail}</small>
          )}
        </div>
      )}
      {clients.length > 0 && (
        <div className="runtime-clients">
          <span>{t("runtime.clients")}</span>
          <code>{clients.join(", ")}</code>
        </div>
      )}
      {lastError && (
        <p className="inline-error" role="alert">
          {lastError}
        </p>
      )}
    </section>
  );
}
