import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Select } from "antd";
import {
  GetDefaultConfig,
  GetStatus,
  StartEngine,
  StopEngine,
  SetLanguage,
  SyncCurrentConfig,
} from "../wailsjs/go/main/App";
import { config, engine } from "../wailsjs/go/models";
import { WindowSetTitle } from "../wailsjs/runtime/runtime";
import {
  DEVICES,
  deviceEnabled,
  deviceEndpoint,
  deviceState,
  toggleDevice,
  type PageId,
} from "./devices";
import ConfigEditor from "./components/ConfigEditor";
import { Toggle } from "./components/ConfigFields";
import DeviceTelemetry from "./components/DeviceTelemetry";
import Icon from "./components/Icon";
import LogViewer from "./components/LogViewer";
import SceneSelector from "./components/SceneSelector";
import TargetEditor from "./components/TargetEditor";
import ErrorMessage from "./components/ErrorMessage";
import ExitConfirmation from "./components/ExitConfirmation";
import { AVAILABLE_LANGUAGES, LANGUAGE_LABELS, changeLanguage, getCurrentLanguage, type SupportedLanguage } from "./i18n";
import "./App.css";

const DEVICE_ICONS = {
  detection: "radio",
  analysis: "analysis",
  precision_df: "compass",
  directed_strike: "crosshair",
  fpv: "video",
  jamming: "wave",
} as const;

type UIError = { cause: unknown; messageKey: string };

export default function App() {
  const { t, i18n } = useTranslation();
  const [cfg, setCfg] = useState<config.Config | null>(null);
  const [status, setStatus] = useState<engine.Status | null>(null);
  const [page, setPage] = useState<PageId>("detection");
  const [action, setAction] = useState<"starting" | "stopping" | null>(null);
  const [error, setError] = useState<UIError | null>(null);
  const [connectionError, setConnectionError] = useState<UIError | null>(null);
  const [notice, setNotice] = useState<{
    message: string;
    type: "success" | "error";
  } | null>(null);
  const cfgRef = useRef<config.Config | null>(null);
  const syncRef = useRef<Promise<void>>(Promise.resolve());
  const syncRevision = useRef(0);
  const requestRef = useRef(0);
  const actionRef = useRef(false);
  const running = status?.running ?? false;
  const locked = running || action !== null;
  const visibleError = error ?? connectionError;

  const notify = (message: string, type: "success" | "error") =>
    setNotice({ message, type });
  const showError = (cause: unknown, messageKey = "errors.syncConfig") => setError({ cause, messageKey });

  const applyConfig = (value: config.Config) => {
    const next = new config.Config(value);
    cfgRef.current = next;
    setCfg(next);
    const revision = ++syncRevision.current;
    syncRef.current = syncRef.current
      .catch(() => undefined)
      .then(() => {
        if (running && revision !== syncRevision.current) return;
        return SyncCurrentConfig(next);
      });
    void syncRef.current.catch(showError);
  };

  const currentConfig = async () => {
    if (document.activeElement instanceof HTMLElement)
      document.activeElement.blur();
    await new Promise<void>((resolve) =>
      window.requestAnimationFrame(() => resolve()),
    );
    await syncRef.current;
    return cfgRef.current;
  };

  const refreshStatus = useCallback(async () => {
    const request = ++requestRef.current;
    const next = await GetStatus();
    if (request === requestRef.current) {
      setStatus(next);
      setConnectionError(null);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    GetDefaultConfig()
      .then((value) => {
        if (cancelled) return;
        const next = new config.Config(value);
        cfgRef.current = next;
        setCfg(next);
      })
      .catch((value) => {
        if (!cancelled) showError(value, "errors.loadConfig");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    let timer: number;
    const poll = async () => {
      try {
        await refreshStatus();
      } catch (value) {
        if (!cancelled) setConnectionError({ cause: value, messageKey: "errors.status" });
      }
      if (!cancelled) timer = window.setTimeout(() => void poll(), 800);
    };
    void poll();
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      requestRef.current++;
    };
  }, [refreshStatus]);

  useEffect(() => {
    document.title = t("app.title");
    document.documentElement.lang = i18n.language;
    WindowSetTitle(t("app.title"));
    void SetLanguage(getCurrentLanguage()).catch((value) => console.error("Unable to sync dialog language", value));
  }, [i18n.language, t]);

  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(null), 3000);
    return () => window.clearTimeout(timer);
  }, [notice]);

  const run = async () => {
    if (actionRef.current || !cfgRef.current) return;
    actionRef.current = true;
    setAction(running ? "stopping" : "starting");
    setError(null);
    try {
      if (running) await StopEngine();
      else {
        const latest = await currentConfig();
        if (latest) await StartEngine(latest);
      }
      await refreshStatus();
    } catch (value) {
      showError(value, running ? "errors.stop" : "errors.start");
    } finally {
      actionRef.current = false;
      setAction(null);
    }
  };

  if (!cfg)
    return (
      <div className="loading" role="status">
        {error ? <ErrorMessage error={error.cause} messageKey={error.messageKey} /> : t("app.loading")}
      </div>
    );

  const enabledCount = DEVICES.filter(({ id }) =>
    deviceEnabled(cfg, id),
  ).length;
  const selectedState =
    page === "targets" ? null : deviceState(cfg, page, status);
  const connections =
    status?.modules?.reduce(
      (sum, item) => sum + (item.connectionCount ?? 0),
      0,
    ) ?? 0;

  return (
    <div className="simulator">
      <ExitConfirmation beforeSave={currentConfig} />
      <aside className="device-sidebar">
        <div className="app-brand">
          <span className="app-brand-icon" aria-hidden="true">
            <svg viewBox="0 0 32 32" fill="none">
              <path
                d="M16 3 28 10v12l-12 7L4 22V10L16 3Z"
                stroke="currentColor"
                strokeWidth="1.6"
              />
              <path
                d="M10 12v7l6 3 6-3v-7M16 9v13M10 12l6 3 6-3"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinejoin="round"
              />
            </svg>
          </span>
          <span>UAV Defender<small>Simulator</small></span>
        </div>
        <div className="sidebar-navigation">
          <div className="sidebar-heading">
            <span>{t("app.modules")}</span>
            <span>
              {enabledCount}/{DEVICES.length}
            </span>
          </div>
          <nav aria-label={t("app.modules")}>
            {DEVICES.map(({ id }) => {
              const state = deviceState(cfg, id, status);
              const endpoint = deviceEndpoint(cfg, id);
              return (
                <div
                  key={id}
                  className={`device-item${page === id ? " is-selected" : ""}${deviceEnabled(cfg, id) ? "" : " is-disabled"}`}
                >
                  <Button type="text"
                    className="device-select"
                    onClick={() => setPage(id)}
                    aria-current={page === id ? "page" : undefined}
                    aria-label={t(`devices.${id}`)}
                    title={t(`devices.${id}`)}
                  >
                    <Icon className="device-icon" name={DEVICE_ICONS[id]} />
                    <span className="device-copy">
                      <span className="device-label">
                        <strong>{t(`devices.${id}`)}</strong>
                        <i className={`state-dot state-${state}`} />
                      </span>
                      <code title={endpoint}>{endpoint || "—"}</code>
                    </span>
                  </Button>
                  <Toggle
                    label={t("config.enableModule", {
                      name: t(`devices.${id}`),
                    })}
                    checked={deviceEnabled(cfg, id)}
                    disabled={locked}
                    onChange={(value) =>
                      applyConfig(toggleDevice(cfg, id, value))
                    }
                  />
                </div>
              );
            })}
          </nav>
          <Button type="text"
            className={`targets-nav${page === "targets" ? " is-selected" : ""}`}
            onClick={() => setPage("targets")}
            aria-current={page === "targets" ? "page" : undefined}
            title={t("config.targetsAndPosition")}
          >
            <Icon name="map" />
            <span>{t("config.targetsAndPosition")}</span>
            <span className="count">{cfg.predefined_drones?.length || ""}</span>
          </Button>
        </div>
        <div className="sidebar-footer">
          <span className={`engine-state${running ? " is-running" : ""}`}>
            <i />
            {t(`app.${action ?? (running ? "running" : "stopped")}`)}
          </span>
          {running && (
            <span>
              {t("app.connections", { count: connections })}
            </span>
          )}
        </div>
      </aside>

      <div className="simulator-main">
        <header className="app-header">
          <div className="page-title">
            <h1>
              {t(
                page === "targets"
                  ? "config.targetsAndPosition"
                  : `devices.${page}`,
              )}
            </h1>
          </div>
          {AVAILABLE_LANGUAGES.length > 1 && (
            <div className="language-select">
              <Icon name="globe" />
              <Select<SupportedLanguage> aria-label={t("app.language")} value={getCurrentLanguage()}
                onChange={(language) => void changeLanguage(language)}
                options={AVAILABLE_LANGUAGES.map((language) => ({ value: language, label: LANGUAGE_LABELS[language] }))} />
            </div>
          )}
        </header>

        <div className="session-toolbar">
          <span className="scene-label">{t("scene.label")}</span>
          <SceneSelector
            currentCfg={cfg}
            getCurrentConfig={currentConfig}
            onLoad={applyConfig}
            onToast={notify}
            running={locked}
          />
          <div className="header-actions">
            <span className={`engine-state${running ? " is-running" : ""}`}>
              <i />
              {t(`app.${action ?? (running ? "running" : "stopped")}`)}
            </span>
            <Button type={running ? "default" : "primary"} danger={running}
              className={`button run-button${running ? " is-stop" : ""}`}
              onClick={() => void run()}
              disabled={action !== null || (!running && enabledCount === 0)}
              title={enabledCount === 0 ? t("config.enableFirst") : undefined}
            >
              <Icon name={running ? "stop" : "play"} />
              {t(`app.${action ?? (running ? "stop" : "start")}`)}
            </Button>
          </div>
        </div>

        {visibleError && (
          <div className="error-banner" role="alert">
            <ErrorMessage error={visibleError.cause} messageKey={visibleError.messageKey} />
            <Button type="text"
              className="icon-button"
              onClick={() => {
                setError(null);
                setConnectionError(null);
              }}
              aria-label={t("app.close")}
            >
              <Icon name="close" />
            </Button>
          </div>
        )}

        <main className="config-workspace">
          <div className="workspace-scroll" key={page}>
            {locked && (
              <p className="read-only">
                {t(
                  page === "detection" && cfg.joint_direction.enabled && running
                    ? "joint.runtimeEditable"
                    : "config.readOnly",
                )}
              </p>
            )}
            {page === "targets" ? (
              <div className="target-config">
                <TargetEditor
                  cfg={cfg}
                  onChange={applyConfig}
                  disabled={locked}
                />
              </div>
            ) : (
              <>
                <section className="config-panel">
                  <div className="workspace-heading">
                    <h2>{t("config.deviceConfig")}</h2>
                    {selectedState && (
                      <span className={`device-state state-${selectedState}`}>
                        <i className={`state-dot state-${selectedState}`} />
                        {t(`states.${selectedState}`)}
                      </span>
                    )}
                    <div className="module-toggle">
                      <span>
                        {t("config.enableModule", {
                          name: t(`devices.${page}`),
                        })}
                      </span>
                      <Toggle
                        label={t("config.enableModule", {
                          name: t(`devices.${page}`),
                        })}
                        checked={deviceEnabled(cfg, page)}
                        disabled={locked}
                        onChange={(value) =>
                          applyConfig(toggleDevice(cfg, page, value))
                        }
                      />
                    </div>
                  </div>
                  <ConfigEditor
                    cfg={cfg}
                    device={page}
                    onChange={applyConfig}
                    onError={showError}
                    disabled={locked}
                    liveJointDirection={running && action === null}
                  />
                </section>
                {deviceEnabled(cfg, page) && (
                  <DeviceTelemetry device={page} status={status} />
                )}
              </>
            )}
          </div>
        </main>
        <LogViewer />
      </div>
      {notice && (
        <div
          className={`notice notice-${notice.type}`}
          role={notice.type === "error" ? "alert" : "status"}
        >
          {notice.type === "error" ? <ErrorMessage error={notice.message} messageKey="errors.scene" /> : t(notice.message)}
        </div>
      )}
    </div>
  );
}
