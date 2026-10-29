import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Select } from "antd";
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
import ErrorMessage from "./ErrorMessage";

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
    "save" | "manage" | "rename" | "delete" | "switch" | "replace" | null
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

  const save = async (overwrite = false) => {
    if (running || !name.trim()) return;
    if (!overwrite) {
      const latestScenes = (await ListScenes()) ?? [];
      setScenes(latestScenes);
      if (latestScenes.includes(name.trim())) {
        setDialog("replace");
        return;
      }
    }
    const latest = await getCurrentConfig();
    if (!latest) return;
    await SaveScene(name.trim(), latest, overwrite);
    baseline.current = JSON.stringify(latest);
    setSelected(name.trim());
    await refresh();
    setDialog(null);
    onToast("scene.saved", "success");
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
    replace: t("scene.replace"),
  };

  return (
    <div className="scene-selector">
      <Select
        className="scene-select"
        aria-label={t("scene.label")}
        value={selected}
        disabled={running || busy}
        onChange={requestLoad}
        options={[{ value: "", label: `${t("scene.default")}${dirty && selected === "" ? " *" : ""}` },
          ...scenes.map((scene) => ({ value: scene, label: `${scene}${dirty && selected === scene ? " *" : ""}` })),
        ]}
      />
      <Button type="text"
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
      </Button>
      <Button type="text"
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
      </Button>
      {dialog && (
        <Modal title={titles[dialog]} onClose={close}>
          {operationError && (
            <div className="dialog-error inline-error" role="alert">
              <ErrorMessage error={operationError} messageKey="errors.scene" />
            </div>
          )}
          {(dialog === "save" || dialog === "rename") && (
            <form
              onSubmit={(event) => {
                event.preventDefault();
                void perform(dialog === "save" ? () => save() : rename);
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
                <Button type="default"
                  htmlType="button"
                  className="button"
                  onClick={close}
                  disabled={busy}
                >
                  {t("app.cancel")}
                </Button>
                <Button type="primary" htmlType="submit"
                  className="button button-primary"
                  disabled={busy || !name.trim()}
                >
                  {t(dialog === "save" ? "scene.save" : "app.confirm")}
                </Button>
              </footer>
            </form>
          )}
          {dialog === "manage" && (
            <>
              <div className="scene-list dialog-body">
                <div className="scene-row">
                  <Button type="text"
                    className={`scene-name${selected === "" ? " is-current" : ""}`}
                    disabled={running || busy}
                    onClick={() => requestLoad("")}
                  >
                    {t("scene.default")}
                  </Button>
                </div>
                {scenes.map((scene) => (
                  <div className="scene-row" key={scene}>
                    <Button type="text"
                      className={`scene-name${scene === selected ? " is-current" : ""}`}
                      disabled={running || busy}
                      onClick={() => requestLoad(scene)}
                    >
                      {scene}
                    </Button>
                    <Button type="text"
                      className="button button-quiet"
                      disabled={running || busy}
                      onClick={() => {
                        setSubject(scene);
                        setName(scene);
                        setDialog("rename");
                      }}
                    >
                      {t("scene.rename")}
                    </Button>
                    <Button type="text"
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
                    </Button>
                    <Button type="text" danger
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
                    </Button>
                  </div>
                ))}
              </div>
              <footer className="dialog-actions">
                <Button type="default"
                  className="button"
                  disabled={running || busy}
                  onClick={() => void perform(importScene)}
                >
                  {t("scene.import")}
                </Button>
                <Button type="default" className="button" onClick={close} disabled={busy}>
                  {t("app.close")}
                </Button>
              </footer>
            </>
          )}
          {(dialog === "delete" || dialog === "switch" || dialog === "replace") && (
            <>
              <div className="dialog-body">
                <p>
                  {dialog === "delete"
                    ? t("scene.confirmDelete", { name: subject })
                    : dialog === "replace" ? t("scene.confirmReplace", { name: name.trim() }) : t("scene.confirmSwitch")}
                </p>
              </div>
              <footer className="dialog-actions">
                <Button type="default" className="button" onClick={() => dialog === "replace" ? setDialog("save") : close()} disabled={busy}>
                  {t("app.cancel")}
                </Button>
                <Button type="primary" danger={dialog === "delete"}
                  className={`button ${dialog === "delete" ? "button-danger" : "button-primary"}`}
                  disabled={busy || running}
                  onClick={() =>
                    void perform(
                      dialog === "delete" ? remove : dialog === "replace" ? () => save(true) : () => load(subject),
                    )
                  }
                >
                  {t(dialog === "delete" ? "scene.delete" : dialog === "replace" ? "scene.replace" : "scene.switch")}
                </Button>
              </footer>
            </>
          )}
        </Modal>
      )}
    </div>
  );
}
