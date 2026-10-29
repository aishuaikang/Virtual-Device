import { useTranslation } from "react-i18next";
import { Collapse } from "antd";
import { engine } from "../../wailsjs/go/models";
import type { DeviceId } from "../devices";
import ErrorMessage from "./ErrorMessage";
import { localizeCommand } from "../i18n/runtime";

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
  const relay = device === "relay16" ? status.relay16 : undefined;
  const activeOutputs = relay
    ? Array.from({ length: 15 }, (_, bit) => (relay.outputMask >> bit) & 1)
        .reduce((sum, value) => sum + value, 0)
    : 0;
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
      : relay
        ? [
            ["RX", relay.receivedFrames],
            ["TX", relay.sentFrames],
            [t("runtime.outputs"), `${activeOutputs}/15`],
          ]
        : [[t("runtime.connections"), module?.connectionCount ?? 0]];
  const lastCommand =
    receiver?.lastCommand ?? directed?.lastCommand ?? relay?.lastCommand;
  const lastError =
    receiver?.lastError ?? directed?.lastError ?? relay?.lastError;
  const clients = module?.clientAddresses ?? [];

  return (
    <section className="telemetry" aria-label={t("runtime.title")}>
      <div className="section-heading">
        <h2>{t("runtime.title")}</h2>
        {receiver?.running && (
          <code>
            {receiver.mode} · {t("runtime.task", { id: receiver.taskID })}
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
      {relay && (
        <div className="relay16-board">
          {Array.from({ length: 5 }, (_, face) => (
            <div className="relay16-face" key={face}>
              <h3>{t("runtime.face", { number: face + 1 })}</h3>
              <div className="relay16-outputs">
                {["433/800/900 MHz", "1.2/1.5 GHz", "2.4/5.2/5.8 GHz"].map(
                  (band, group) => {
                    const channel = face * 3 + group + 1;
                    const enabled = (relay.outputMask & (1 << (channel - 1))) !== 0;
                    const state = t(
                      enabled ? "runtime.outputOn" : "runtime.outputOff",
                    );
                    return (
                      <span
                        className="relay16-coil"
                        key={channel}
                        aria-label={`IO${channel}: ${state}`}
                      >
                        <code>IO{channel}</code>
                        <i
                          className={`state-dot state-${enabled ? "connected" : "disabled"}`}
                        />
                        <span>{state}</span>
                        <small>{band}</small>
                      </span>
                    );
                  },
                )}
              </div>
            </div>
          ))}
          <div className="relay16-summary">
            <span>
              {t("runtime.reservedOutput")} · {t(
                relay.outputMask & 0x8000 ? "runtime.outputOn" : "runtime.outputOff",
              )}
            </span>
            <code>
              0x{relay.outputMask.toString(16).toUpperCase().padStart(4, "0")}
            </code>
          </div>
        </div>
      )}
      {lastCommand && (
        <div className="last-command">
          <span>{t("runtime.lastCommand")}</span>
          <code>{localizeCommand(lastCommand, t)}</code>
          {directed?.lastCommandDetail && (
            <Collapse className="runtime-command-details" ghost size="small" items={[{
              key: "details", label: t("app.details"), children: <pre>{directed.lastCommandDetail}</pre>,
            }]} />
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
        <div className="inline-error" role="alert">
          <ErrorMessage error={lastError} />
        </div>
      )}
    </section>
  );
}
