import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  GetDefaultConfig,
  GetStatus,
  StartEngine,
  StopEngine,
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
import "./App.css";

function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export default function App() {
  const { t, i18n } = useTranslation();
  const [cfg, setCfg] = useState<config.Config | null>(null);
  const [status, setStatus] = useState<engine.Status | null>(null);
  const [page, setPage] = useState<PageId>("detection");
  const [action, setAction] = useState<"starting" | "stopping" | null>(null);
  const [error, setError] = useState("");
  const [connectionError, setConnectionError] = useState("");
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

  const notify = (message: string, type: "success" | "error") =>
    setNotice({ message, type });
  const showError = (value: unknown) => setError(errorText(value));

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
      setConnectionError("");
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
        if (!cancelled) showError(value);
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
        if (!cancelled) setConnectionError(errorText(value));
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
    setError("");
    try {
      if (running) await StopEngine();
      else {
        const latest = await currentConfig();
        if (latest) await StartEngine(latest);
      }
      await refreshStatus();
    } catch (value) {
      showError(value);
    } finally {
      actionRef.current = false;
      setAction(null);
    }
  };

  if (!cfg)
    return (
      <div className="loading" role="status">
        {error || t("app.loading")}
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
      <header className="app-header">
        <div className="app-brand">
          <Icon name="radio" />
          <strong>{t("app.title")}</strong>
        </div>
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
          <button
            className="button button-quiet language-button"
            onClick={() =>
              void i18n.changeLanguage(i18n.language === "zh" ? "en" : "zh")
            }
            aria-label={t("app.language")}
          >
            {i18n.language === "zh" ? "EN" : "中文"}
          </button>
          <button
            className={`button run-button${running ? " is-stop" : ""}`}
            onClick={() => void run()}
            disabled={action !== null || (!running && enabledCount === 0)}
            title={enabledCount === 0 ? t("config.enableFirst") : undefined}
          >
            <Icon name={running ? "stop" : "play"} />
            {t(`app.${action ?? (running ? "stop" : "start")}`)}
          </button>
        </div>
      </header>

      {(error || connectionError) && (
        <div className="error-banner" role="alert">
          <span>{error || connectionError}</span>
          <button
            className="icon-button"
            onClick={() => {
              setError("");
              setConnectionError("");
            }}
            aria-label={t("app.close")}
          >
            <Icon name="close" />
          </button>
        </div>
      )}

      <div className="workbench">
        <aside className="device-sidebar">
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
                  <button
                    className="device-select"
                    onClick={() => setPage(id)}
                    aria-current={page === id ? "page" : undefined}
                    aria-label={t(`devices.${id}`)}
                  >
                    <span className="device-label">
                      <i className={`state-dot state-${state}`} />
                      <strong>{t(`devices.${id}`)}</strong>
                    </span>
                    <code title={endpoint}>{endpoint || "—"}</code>
                  </button>
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
          <button
            className={`targets-nav${page === "targets" ? " is-selected" : ""}`}
            onClick={() => setPage("targets")}
            aria-current={page === "targets" ? "page" : undefined}
          >
            <Icon name="map" />
            <span>{t("config.targetsAndPosition")}</span>
            <span className="count">{cfg.predefined_drones?.length || ""}</span>
          </button>
          {running && (
            <div className="sidebar-footer">
              {t("app.connections", { count: connections })}
            </div>
          )}
        </aside>

        <main className="config-workspace">
          <div className="workspace-heading">
            <h1>
              {t(
                page === "targets"
                  ? "config.targetsAndPosition"
                  : `devices.${page}`,
              )}
            </h1>
            {selectedState && (
              <span className={`device-state state-${selectedState}`}>
                <i className={`state-dot state-${selectedState}`} />
                {t(`states.${selectedState}`)}
              </span>
            )}
            {locked && (
              <span className="read-only">
                {t(
                  page === "detection" && cfg.joint_direction.enabled && running
                    ? "joint.runtimeEditable"
                    : "config.readOnly",
                )}
              </span>
            )}
          </div>
          <div className="workspace-scroll" key={page}>
            {page === "targets" ? (
              <TargetEditor
                cfg={cfg}
                onChange={applyConfig}
                disabled={locked}
              />
            ) : (
              <>
                <ConfigEditor
                  cfg={cfg}
                  device={page}
                  onChange={applyConfig}
                  onError={showError}
                  disabled={locked}
                  liveJointDirection={running && action === null}
                />
                {deviceEnabled(cfg, page) && (
                  <DeviceTelemetry device={page} status={status} />
                )}
              </>
            )}
          </div>
        </main>
      </div>
      <LogViewer />
      {notice && (
        <div
          className={`notice notice-${notice.type}`}
          role={notice.type === "error" ? "alert" : "status"}
        >
          {notice.message}
        </div>
      )}
    </div>
  );
}
