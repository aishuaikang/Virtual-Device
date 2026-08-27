import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { config, engine } from "../../wailsjs/go/models";
import { GetLocalIPs } from "../../wailsjs/go/main/App";

type EngineAction = "starting" | "stopping" | null;

interface Props {
  cfg: config.Config;
  status: engine.Status | null;
  actionState: EngineAction;
}

const MODULE_KEYS = ["analysis", "detection", "fpv", "jamming"] as const;

function formatEndpoint(host: string, port: number) {
  return host.includes(":") ? `[${host}]:${port}` : `${host}:${port}`;
}

function isWildcardHost(host: string) {
  return host === "" || host === "0.0.0.0" || host === "::";
}

export default function OperationGuide({ cfg, status, actionState }: Props) {
  const { t } = useTranslation();
  const [localIPs, setLocalIPs] = useState<string[]>([]);
  const running = status?.running ?? false;
  const directed = status?.directedStrike;
  const directedEnabled = cfg.directed_strike?.enabled !== false;

  useEffect(() => {
    let active = true;
    GetLocalIPs()
      .then((ips) => {
        if (active) setLocalIPs(ips ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  const detectionEnabled = cfg.detections?.length
    ? cfg.detections.some((item) => item.enabled !== false)
    : cfg.detection?.enabled !== false;
  const enabledModules = [
    cfg.analysis?.enabled !== false,
    detectionEnabled,
    cfg.fpv?.enabled !== false,
    cfg.jamming?.enabled !== false,
    directedEnabled,
  ].filter(Boolean).length;
  const totalConnections =
    status?.modules?.reduce(
      (total, item) => total + (item.connectionCount ?? 0),
      0,
    ) ?? 0;
  const exchangedFrames =
    (directed?.receivedFrames ?? 0) + (directed?.sentFrames ?? 0);
  const directedConnections = directed?.activeConnections ?? 0;
  const anyConnected = totalConnections > 0;
  const workflowConnections = directedEnabled
    ? directedConnections
    : totalConnections;
  const connected = workflowConnections > 0;
  const interacting = directedEnabled && exchangedFrames > 0;
  const hasIssue = directedEnabled && Boolean(directed?.lastError);
  const bindHost = cfg.directed_strike?.host?.trim() || "0.0.0.0";
  const listenPort = cfg.directed_strike?.port || 19000;
  const connectHost = isWildcardHost(bindHost) ? localIPs[0] : bindHost;
  const endpoint = connectHost
    ? formatEndpoint(connectHost, listenPort)
    : t("overview.localEndpoint", { port: listenPort });
  const directedReady = directedEnabled && Boolean(directed?.listening);

  const state = hasIssue
    ? "issue"
    : interacting
      ? "verified"
      : connected
        ? "connected"
        : running
          ? "waiting"
          : "idle";
  const stateLabel = t(`overview.state.${state}`);
  const heroTitle = t(`overview.hero.${state}Title`);
  const heroDetailKey =
    state === "waiting" && !directedEnabled
      ? "overview.hero.waitingGenericDetail"
      : `overview.hero.${state}Detail`;
  const heroDetail = t(heroDetailKey, {
    count: workflowConnections,
    endpoint,
  });

  const steps = [
    {
      number: "01",
      title: t("overview.steps.configureTitle"),
      detail: t("overview.steps.configureDetail"),
      state: running ? "complete" : "active",
    },
    {
      number: "02",
      title: t("overview.steps.connectTitle"),
      detail: directedEnabled
        ? directedReady
          ? t("overview.steps.connectDetail", { endpoint })
          : t("overview.steps.connectPrepared", { endpoint })
        : t("overview.steps.connectGeneric"),
      state: connected ? "complete" : running ? "active" : "pending",
    },
    {
      number: "03",
      title: t("overview.steps.verifyTitle"),
      detail: t("overview.steps.verifyDetail"),
      state: interacting ? "complete" : connected ? "active" : "pending",
    },
  ];

  const enabledModuleLabels = MODULE_KEYS.filter((key) => {
    if (key === "detection") return detectionEnabled;
    return cfg[key]?.enabled !== false;
  }).map((key) => t(`status.${key}`));
  if (directedEnabled) enabledModuleLabels.push(t("status.directedStrike"));

  return (
    <section className={`operation-guide is-${state}`}>
      <div className="guide-header">
        <div>
          <span className="panel-eyebrow">GUIDED OPERATION</span>
          <h2>{t("overview.title")}</h2>
        </div>
        <span className="guide-state">
          <i />
          {actionState === "starting"
            ? t("app.starting")
            : actionState === "stopping"
              ? t("app.stopping")
              : stateLabel}
        </span>
      </div>

      <div className="guide-hero" aria-live="polite">
        <div className="guide-hero-mark" aria-hidden="true">
          <span />
        </div>
        <div className="guide-hero-copy">
          <span>{t("overview.nextAction")}</span>
          <strong>{heroTitle}</strong>
          <p>{hasIssue ? directed?.lastError : heroDetail}</p>
        </div>
        {directedReady && (
          <div className="guide-endpoint">
            <span>TCP</span>
            <code>{endpoint}</code>
          </div>
        )}
      </div>

      <ol className="guide-steps" aria-label={t("overview.stepsLabel")}>
        {steps.map((step) => (
          <li key={step.number} className={`is-${step.state}`}>
            <span className="guide-step-number">{step.number}</span>
            <div>
              <strong>{step.title}</strong>
              <small>{step.detail}</small>
            </div>
            <span className="guide-step-state" aria-hidden="true">
              {step.state === "complete" ? "✓" : ""}
            </span>
          </li>
        ))}
      </ol>

      <div className="guide-metrics">
        <article>
          <span>{t("overview.metrics.modules")}</span>
          <strong>{enabledModules}</strong>
          <small>{enabledModuleLabels.join(" · ")}</small>
        </article>
        <article>
          <span>{t("overview.metrics.connections")}</span>
          <strong>{totalConnections}</strong>
          <small>
            {anyConnected
              ? t("overview.metrics.connectionsReady")
              : t("overview.metrics.connectionsWaiting")}
          </small>
        </article>
        <article>
          <span>{t("overview.metrics.targets")}</span>
          <strong>{status?.droneCount ?? 0}</strong>
          <small>{t("overview.metrics.targetsDetail")}</small>
        </article>
        <article>
          <span>{t("overview.metrics.frames")}</span>
          <strong>{exchangedFrames}</strong>
          <small>RX {directed?.receivedFrames ?? 0} · TX {directed?.sentFrames ?? 0}</small>
        </article>
      </div>

      <div className="guide-latest">
        <span className="guide-latest-icon" aria-hidden="true" />
        <div>
          <span>{t("overview.latest")}</span>
          <strong>
            {directed?.lastCommand || t("overview.noInteraction")}
          </strong>
          <small>
            {directed?.lastCommandDetail || t("overview.noInteractionDetail")}
          </small>
        </div>
      </div>
    </section>
  );
}
