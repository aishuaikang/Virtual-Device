import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import "./i18n";
import {
  GetDefaultConfig,
  GetStatus,
  StartEngine,
  StopEngine,
  SyncCurrentConfig,
} from "../wailsjs/go/main/App";
import { config, engine } from "../wailsjs/go/models";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { WindowSetTitle } from "../wailsjs/runtime/runtime";
import ConfigEditor from "./components/ConfigEditor";
import DirectedStrikePanel from "./components/DirectedStrikePanel";
import ModuleStatus from "./components/ModuleStatus";
import OperationGuide from "./components/OperationGuide";
import SceneSelector from "./components/SceneSelector";
import "./App.css";

interface Toast {
  id: number;
  msg: string;
  type: "success" | "error";
}

type EngineAction = "starting" | "stopping" | null;

function formatError(error: unknown) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return String(error);
}

export default function App() {
  const { t, i18n } = useTranslation();
  const [cfg, setCfg] = useState<config.Config | null>(null);
  const [status, setStatus] = useState<engine.Status | null>(null);
  const [error, setError] = useState("");
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [engineAction, setEngineAction] = useState<EngineAction>(null);
  const toastId = useRef(0);
  const cfgRef = useRef<config.Config | null>(null);
  const pollingRef = useRef(false);
  const statusRefreshTimerRef = useRef<number | null>(null);
  const configSyncPromiseRef = useRef<Promise<void>>(Promise.resolve());
  const configSyncQueuedRef = useRef(false);

  cfgRef.current = cfg;

  const pushToast = (msg: string, type: "success" | "error") => {
    const id = ++toastId.current;
    setToasts((prev) => [...prev, { id, msg, type }]);
    setTimeout(
      () => setToasts((prev) => prev.filter((t) => t.id !== id)),
      3000,
    );
  };

  const running = status?.running ?? false;

  const scheduleConfigSync = () => {
    if (configSyncQueuedRef.current) {
      return configSyncPromiseRef.current;
    }

    configSyncQueuedRef.current = true;
    configSyncPromiseRef.current = configSyncPromiseRef.current
      .catch(() => undefined)
      .then(async () => {
        configSyncQueuedRef.current = false;
        const latestCfg = cfgRef.current;
        if (!latestCfg) {
          return;
        }

        try {
          await SyncCurrentConfig(latestCfg);
        } catch {
          // Ignore non-Wails environments.
        }
      });

    return configSyncPromiseRef.current;
  };

  const applyCfg = (nextCfg: config.Config) => {
    cfgRef.current = nextCfg;
    setCfg(nextCfg);
    void scheduleConfigSync();
  };

  useEffect(() => {
    let cancelled = false;

    GetDefaultConfig()
      .then((nextCfg) => {
        if (!cancelled) {
          applyCfg(nextCfg);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setError(`${i18n.t("error.loadConfigFailed")}: ${formatError(error)}`);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const refreshStatus = async () => {
    const nextStatus = await GetStatus();
    setStatus(nextStatus);
    return nextStatus;
  };

  const scheduleStatusRefresh = (delay = 200) => {
    if (statusRefreshTimerRef.current !== null) {
      return;
    }

    statusRefreshTimerRef.current = window.setTimeout(() => {
      statusRefreshTimerRef.current = null;
      if (pollingRef.current) {
        return;
      }

      pollingRef.current = true;
      void refreshStatus()
        .catch((error) => {
          console.error("Failed to refresh engine status:", error);
        })
        .finally(() => {
          pollingRef.current = false;
        });
    }, delay);
  };

  useEffect(() => {
    let cancelled = false;

    const tick = async () => {
      if (cancelled || pollingRef.current) {
        return;
      }

      pollingRef.current = true;
      try {
        await refreshStatus();
      } catch (error) {
        if (!cancelled) {
          console.error("Failed to refresh engine status:", error);
        }
      } finally {
        pollingRef.current = false;
      }

      if (!cancelled) {
        window.setTimeout(() => {
          void tick();
        }, 1500);
      }
    };

    void tick();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const moduleLogPattern =
      /\[(?:侦测模块_UDP服务|解析模块|FPV模块|干扰模块|定向打击)\]/;
    const activityPattern =
      /连接成功|新客户端连接|客户端已连接|客户端已断开|\b(?:RX|TX)\b|接收到来自|接收到数据|发送响应|发送侦测数据|发送告警数据|清理不活跃客户端|读取数据从 .* EOF/;

    try {
      const cancel = EventsOn("log", (line: string) => {
        if (!moduleLogPattern.test(line) || !activityPattern.test(line)) {
          return;
        }
        scheduleStatusRefresh();
      });
      return () => {
        if (statusRefreshTimerRef.current !== null) {
          window.clearTimeout(statusRefreshTimerRef.current);
          statusRefreshTimerRef.current = null;
        }
        cancel();
      };
    } catch {
      return () => {
        if (statusRefreshTimerRef.current !== null) {
          window.clearTimeout(statusRefreshTimerRef.current);
          statusRefreshTimerRef.current = null;
        }
      };
    }
  }, []);

  useEffect(() => {
    const title = t("app.title");
    document.title = title;
    document.documentElement.lang = i18n.language;

    try {
      WindowSetTitle(title);
    } catch {
      // Ignore non-Wails environments.
    }
  }, [i18n.language, t]);

  useEffect(() => {
    if (engineAction === "starting" && running) {
      setEngineAction(null);
    }
    if (engineAction === "stopping" && !running) {
      setEngineAction(null);
    }
  }, [engineAction, running]);

  const flushActiveField = async () => {
    const active = document.activeElement;
    if (active instanceof HTMLElement) {
      active.blur();
    }

    await new Promise<void>((resolve) => {
      window.requestAnimationFrame(() => resolve());
    });
  };

  const handleStart = async () => {
    if (!cfgRef.current || engineAction) return;

    await flushActiveField();

    const latestCfg = cfgRef.current;
    if (!latestCfg) return;

    setEngineAction("starting");
    setError("");
    try {
      await scheduleConfigSync();
      await StartEngine(latestCfg);
      await refreshStatus();
      pushToast(t("app.startSuccess"), "success");
    } catch (e) {
      const msg = `${t("error.startFailed")}: ${formatError(e)}`;
      setError(msg);
      pushToast(msg, "error");
      setEngineAction(null);
    }
  };

  const handleStop = async () => {
    if (engineAction) return;
    setEngineAction("stopping");
    try {
      await StopEngine();
      await refreshStatus();
      pushToast(t("app.stopSuccess"), "success");
    } catch (e) {
      const msg = `${t("error.stopFailed")}: ${formatError(e)}`;
      setError(msg);
      pushToast(msg, "error");
      setEngineAction(null);
    }
  };

  const toggleLang = () =>
    void i18n.changeLanguage(i18n.language === "zh" ? "en" : "zh");

  if (!cfg) return <div className="loading">{t("app.loading")}</div>;

  const directedConnections = status?.directedStrike?.activeConnections ?? 0;
  const directedListening = Boolean(status?.directedStrike?.listening);
  const directedEnabled = cfg.directed_strike?.enabled !== false;
  const totalConnections =
    status?.modules?.reduce(
      (total, module) => total + (module.connectionCount ?? 0),
      0,
    ) ?? 0;
  const workflowConnections = directedEnabled
    ? directedConnections
    : totalConnections;
  const workflowListening = directedEnabled ? directedListening : running;
  const runButtonLabel =
    engineAction === "starting"
      ? t("app.starting")
      : engineAction === "stopping"
        ? t("app.stopping")
        : running
          ? t("app.stopSimulation")
          : t("app.startSimulation");

  return (
    <div className="app-layout">
      <header className="topbar">
        <div className="brand-lockup">
          <span className="brand-mark" aria-hidden="true">
            <i />
            <b />
          </span>
          <div>
            <strong>{t("app.title")}</strong>
            <small>{t("app.subtitle")}</small>
          </div>
        </div>

        <div className="workflow-strip" aria-label={t("app.workflow")}>
          <span className={!running ? "is-active" : "is-complete"}>
            <b>01</b> {t("app.configure")}
          </span>
          <i />
          <span
            className={
              running && workflowConnections === 0
                ? "is-active"
                : running
                  ? "is-complete"
                  : ""
            }
          >
            <b>02</b> {t("app.listen")}
          </span>
          <i />
          <span className={workflowConnections > 0 ? "is-active" : ""}>
            <b>03</b> {t("app.observe")}
          </span>
        </div>

        <div className="topbar-actions">
          <div
            className={`endpoint-summary${workflowConnections > 0 ? " is-online" : ""}`}
          >
            <span className="endpoint-summary-dot" />
            <span>
              {workflowConnections > 0
                ? t(
                    directedEnabled
                      ? "app.directedClients"
                      : "app.activeConnections",
                    { count: workflowConnections },
                  )
                : workflowListening
                  ? t(
                      directedEnabled
                        ? "app.waitingConnection"
                        : "app.waitingAnyConnection",
                    )
                  : t("app.waitingStart")}
            </span>
          </div>
          <SceneSelector
            currentCfg={cfg}
            onLoad={applyCfg}
            onToast={pushToast}
            running={running}
          />
          <button className="btn btn-ghost" onClick={toggleLang}>
            {i18n.language === "zh" ? "EN" : "中文"}
          </button>
          <button
            className={`run-button${running ? " is-stop" : ""}${engineAction ? " is-pending" : ""}`}
            onClick={running ? handleStop : handleStart}
            disabled={engineAction !== null}
          >
            <span className="run-button-icon" aria-hidden="true">
              {running ? "■" : "▶"}
            </span>
            {runButtonLabel}
          </button>
        </div>
      </header>

      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button className="error-close" onClick={() => setError("")}>
            ✕
          </button>
        </div>
      )}

      <div className="main-content">
        <aside className="left-panel">
          <div className="panel-header">
            <span>
              <b>01</b> {t("config.title")}
            </span>
            {running && (
              <span className="panel-header-badge">{t("config.readOnly")}</span>
            )}
          </div>
          <div className="panel-scroll">
            <ConfigEditor cfg={cfg} onChange={applyCfg} disabled={running} />
          </div>
        </aside>

        <main className="runtime-panel">
          <ModuleStatus
            cfg={cfg}
            status={status}
            actionState={engineAction}
          />
          <OperationGuide
            cfg={cfg}
            status={status}
            actionState={engineAction}
          />
        </main>

        <aside className="operations-panel">
          <DirectedStrikePanel
            config={cfg.directed_strike}
            snapshot={status?.directedStrike}
            running={running}
          />
        </aside>
      </div>

      <div className="toast-container">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast toast-${toast.type}`}>
            {toast.msg}
          </div>
        ))}
      </div>
    </div>
  );
}
