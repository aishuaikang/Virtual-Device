import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  DeleteScene,
  ExportScene,
  GetDefaultConfig,
  ImportScene,
  ListScenes,
  LoadScene,
  RenameScene,
  SaveScene,
} from "../../wailsjs/go/main/App";
import { config } from "../../wailsjs/go/models";
import { ClearableTextInput } from "./ClearableInput";
import Icon from "./Icon";
import Modal from "./Modal";

interface Props {
  currentCfg: config.Config;
  getCurrentConfig: () => Promise<config.Config | null>;
  onLoad: (cfg: config.Config) => void;
  onToast: (message: string, type: "success" | "error") => void;
  running: boolean;
}

export default function SceneSelector({
  currentCfg,
  getCurrentConfig,
  onLoad,
  onToast,
  running,
}: Props) {
  const { t } = useTranslation();
  const [scenes, setScenes] = useState<string[]>([]);
  const [selected, setSelected] = useState("");
  const [dialog, setDialog] = useState<
    "save" | "manage" | "rename" | "delete" | "switch" | null
  >(null);
  const [name, setName] = useState("");
  const [subject, setSubject] = useState("");
  const [busy, setBusy] = useState(false);
  const [operationError, setOperationError] = useState("");
  const baseline = useRef(JSON.stringify(currentCfg));
  const dirty = JSON.stringify(currentCfg) !== baseline.current;

  const refresh = async () => setScenes((await ListScenes()) ?? []);
  useEffect(() => {
    void refresh().catch((error) => onToast(String(error), "error"));
  }, []);
  useEffect(() => setOperationError(""), [dialog]);

  const perform = async (operation: () => Promise<void>) => {
    if (busy) return;
    setBusy(true);
    setOperationError("");
    try {
      await operation();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (!/cancel|已取消/i.test(message)) {
        if (dialog) setOperationError(message);
        else onToast(message, "error");
      }
    } finally {
      setBusy(false);
    }
  };

  const applyScene = (value: config.Config, sceneName: string) => {
    const next = new config.Config(value);
    baseline.current = JSON.stringify(next);
    setSelected(sceneName);
    onLoad(next);
    setDialog(null);
  };

  const load = async (sceneName: string) => {
    if (running) return;
    const next = sceneName
      ? await LoadScene(sceneName)
      : await GetDefaultConfig();
    applyScene(next, sceneName);
  };

  const requestLoad = (sceneName: string) => {
    if (running) return;
    if (dirty) {
      setSubject(sceneName);
      setDialog("switch");
    } else void perform(() => load(sceneName));
  };

  const save = async () => {
    if (running || !name.trim()) return;
    const latest = await getCurrentConfig();
    if (!latest) return;
    await SaveScene(name.trim(), latest);
    baseline.current = JSON.stringify(latest);
    setSelected(name.trim());
    await refresh();
    setDialog(null);
    onToast(t("scene.saved"), "success");
  };

  const importScene = async () => {
    if (running) return;
    const imported = await ImportScene();
    if (!imported) return;
    await refresh();
    if (dirty) {
      setSubject(imported);
      setDialog("switch");
    } else await load(imported);
  };

  const rename = async () => {
    if (running || !name.trim()) return;
    await RenameScene(subject, name.trim());
    if (selected === subject) setSelected(name.trim());
    await refresh();
    setDialog("manage");
  };

  const remove = async () => {
    if (running) return;
    await DeleteScene(subject);
    if (selected === subject) setSelected("");
    await refresh();
    setDialog("manage");
  };

  const close = () => {
    if (!busy) setDialog(null);
  };
  const titles = {
    save: t("scene.save"),
    manage: t("scene.manage"),
    rename: t("scene.rename"),
    delete: t("scene.delete"),
    switch: t("scene.switch"),
  };

  return (
    <div className="scene-selector">
      <select
        className="scene-select"
        aria-label={t("scene.label")}
        value={selected}
        disabled={running || busy}
        onChange={(event) => requestLoad(event.target.value)}
      >
        <option value="">
          {t("scene.default")}
          {dirty && selected === "" ? " *" : ""}
        </option>
        {scenes.map((scene) => (
          <option key={scene} value={scene}>
            {scene}
            {dirty && selected === scene ? " *" : ""}
          </option>
        ))}
      </select>
      <button
        className="icon-button"
        title={t("scene.save")}
        aria-label={t("scene.save")}
        disabled={running || busy}
        onClick={() => {
          setName(selected);
          setDialog("save");
        }}
      >
        <Icon name="save" />
      </button>
      <button
        className="icon-button"
        title={t("scene.manage")}
        aria-label={t("scene.manage")}
        disabled={busy}
        onClick={() =>
          void perform(async () => {
            await refresh();
            setDialog("manage");
          })
        }
      >
        <Icon name="more" />
      </button>
      {dialog && (
        <Modal title={titles[dialog]} onClose={close}>
          {operationError && (
            <p className="dialog-error inline-error" role="alert">
              {operationError}
            </p>
          )}
          {(dialog === "save" || dialog === "rename") && (
            <form
              onSubmit={(event) => {
                event.preventDefault();
                void perform(dialog === "save" ? save : rename);
              }}
            >
              <div className="dialog-body">
                <ClearableTextInput
                  autoFocus
                  value={name}
                  onValueChange={setName}
                  aria-label={t("scene.name")}
                  placeholder={t("scene.name")}
                  disabled={busy}
                  required
                />
              </div>
              <footer className="dialog-actions">
                <button
                  type="button"
                  className="button"
                  onClick={close}
                  disabled={busy}
                >
                  {t("app.cancel")}
                </button>
                <button
                  className="button button-primary"
                  disabled={busy || !name.trim()}
                >
                  {t(dialog === "save" ? "scene.save" : "app.confirm")}
                </button>
              </footer>
            </form>
          )}
          {dialog === "manage" && (
            <>
              <div className="scene-list dialog-body">
                <div className="scene-row">
                  <button
                    className={`scene-name${selected === "" ? " is-current" : ""}`}
                    disabled={running || busy}
                    onClick={() => requestLoad("")}
                  >
                    {t("scene.default")}
                  </button>
                </div>
                {scenes.map((scene) => (
                  <div className="scene-row" key={scene}>
                    <button
                      className={`scene-name${scene === selected ? " is-current" : ""}`}
                      disabled={running || busy}
                      onClick={() => requestLoad(scene)}
                    >
                      {scene}
                    </button>
                    <button
                      className="button button-quiet"
                      disabled={running || busy}
                      onClick={() => {
                        setSubject(scene);
                        setName(scene);
                        setDialog("rename");
                      }}
                    >
                      {t("scene.rename")}
                    </button>
                    <button
                      className="icon-button"
                      disabled={busy}
                      title={t("scene.export")}
                      aria-label={`${t("scene.export")} ${scene}`}
                      onClick={() =>
                        void perform(async () => {
                          await ExportScene(scene);
                        })
                      }
                    >
                      <Icon name="download" />
                    </button>
                    <button
                      className="icon-button danger"
                      disabled={running || busy}
                      title={t("scene.delete")}
                      aria-label={`${t("scene.delete")} ${scene}`}
                      onClick={() => {
                        setSubject(scene);
                        setDialog("delete");
                      }}
                    >
                      <Icon name="trash" />
                    </button>
                  </div>
                ))}
              </div>
              <footer className="dialog-actions">
                <button
                  className="button"
                  disabled={running || busy}
                  onClick={() => void perform(importScene)}
                >
                  {t("scene.import")}
                </button>
                <button className="button" onClick={close} disabled={busy}>
                  {t("app.close")}
                </button>
              </footer>
            </>
          )}
          {(dialog === "delete" || dialog === "switch") && (
            <>
              <div className="dialog-body">
                <p>
                  {dialog === "delete"
                    ? t("scene.confirmDelete", { name: subject })
                    : t("scene.confirmSwitch")}
                </p>
              </div>
              <footer className="dialog-actions">
                <button className="button" onClick={close} disabled={busy}>
                  {t("app.cancel")}
                </button>
                <button
                  className={`button ${dialog === "delete" ? "button-danger" : "button-primary"}`}
                  disabled={busy || running}
                  onClick={() =>
                    void perform(
                      dialog === "delete" ? remove : () => load(subject),
                    )
                  }
                >
                  {t(dialog === "delete" ? "scene.delete" : "scene.switch")}
                </button>
              </footer>
            </>
          )}
        </Modal>
      )}
    </div>
  );
}
