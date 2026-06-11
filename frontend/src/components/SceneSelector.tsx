import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  ExportScene,
  ImportScene,
  ListScenes,
  LoadScene,
  SaveScene,
  DeleteScene,
  RenameScene,
} from "../../wailsjs/go/main/App";
import { config } from "../../wailsjs/go/models";
import { ClearableTextInput } from "./ClearableInput";

interface Props {
  currentCfg: config.Config;
  onLoad: (cfg: config.Config) => void;
  onToast: (msg: string, type: "success" | "error") => void;
  running: boolean;
}

function formatError(error: unknown) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return String(error);
}

function isCancelled(error: unknown) {
  const message = formatError(error).toLowerCase();
  return message.includes("已取消保存") || message.includes("cancel");
}

export default function SceneSelector({
  currentCfg,
  onLoad,
  onToast,
  running,
}: Props) {
  const { t } = useTranslation();
  const [scenes, setScenes] = useState<string[]>([]);
  const [selected, setSelected] = useState("");
  const [open, setOpen] = useState(false);
  const [renaming, setRenaming] = useState("");
  const [newName, setNewName] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveName, setSaveName] = useState("");
  const [pendingDelete, setPendingDelete] = useState("");
  const dropRef = useRef<HTMLDivElement>(null);

  const closeSceneModal = () => {
    setOpen(false);
    setRenaming("");
    setNewName("");
    setPendingDelete("");
  };

  const refresh = async () => {
    const list = await ListScenes();
    setScenes(list ?? []);
  };

  const handleOpen = async () => {
    await refresh();
    setOpen(true);
  };

  const handleLoad = async (name: string) => {
    if (running) return;
    try {
      const cfg = await LoadScene(name);
      if (cfg) {
        onLoad(cfg);
        setSelected(name);
      }
      closeSceneModal();
    } catch (error) {
      onToast(`${t("error.loadSceneFailed")}: ${formatError(error)}`, "error");
    }
  };

  const handleSave = async () => {
    if (running || !saveName.trim()) return;
    try {
      await SaveScene(saveName.trim(), currentCfg);
      setSelected(saveName.trim());
      onToast(t("scene.saved"), "success");
      await refresh();
      setSaving(false);
      setSaveName("");
    } catch (error) {
      if (!isCancelled(error)) {
        onToast(`${t("error.saveFailed")}: ${formatError(error)}`, "error");
      }
    }
  };

  const handleImport = async () => {
    if (running) return;
    try {
      const importedName = await ImportScene();
      if (!importedName) return;

      await refresh();
      const importedCfg = await LoadScene(importedName);
      if (importedCfg) {
        onLoad(importedCfg);
        setSelected(importedName);
      }
      closeSceneModal();
      onToast(t("scene.imported", { name: importedName }), "success");
    } catch (error) {
      onToast(`${t("error.importFailed")}: ${formatError(error)}`, "error");
    }
  };

  const handleDelete = async () => {
    if (running || !pendingDelete) return;

    try {
      await DeleteScene(pendingDelete);
      await refresh();
      if (selected === pendingDelete) setSelected("");
      onToast(t("scene.deleted"), "success");
      setPendingDelete("");
    } catch (error) {
      onToast(`${t("error.deleteFailed")}: ${formatError(error)}`, "error");
    }
  };

  const handleRename = async (oldName: string) => {
    if (running) {
      setRenaming("");
      setNewName("");
      return;
    }
    const trimmed = newName.trim();
    if (!trimmed || trimmed === oldName) {
      setRenaming("");
      setNewName("");
      return;
    }
    try {
      await RenameScene(oldName, trimmed);
      await refresh();
      if (selected === oldName) setSelected(trimmed);
      onToast(t("scene.renamed"), "success");
      setRenaming("");
      setNewName("");
    } catch (error) {
      onToast(`${t("error.renameFailed")}: ${formatError(error)}`, "error");
    }
  };

  const handleExport = async (name: string) => {
    try {
      const exportPath = await ExportScene(name);
      if (!exportPath) return;
      onToast(t("scene.exported", { name }), "success");
    } catch (error) {
      onToast(`${t("error.exportFailed")}: ${formatError(error)}`, "error");
    }
  };

  // 点击外部关闭下拉
  useEffect(() => {
    if (!open && !saving) return;
    const handler = (e: MouseEvent) => {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) {
        closeSceneModal();
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open, saving]);

  return (
    <div className="scene-selector" ref={dropRef}>
      <div className="scene-btn-group">
        <button
          type="button"
          className="btn btn-secondary scene-main-btn"
          onClick={handleOpen}
        >
          {t("scene.label")}
          {selected ? `: ${selected}` : ""}
        </button>
        <button
          type="button"
          className="btn btn-secondary scene-save-btn"
          title={t("scene.save")}
          disabled={running}
          onClick={() => {
            setSaving(true);
            setSaveName(selected || "");
          }}
        >
          &#x1F4BE;
        </button>
      </div>

      {saving && (
        <div className="modal-overlay" onClick={() => setSaving(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t("scene.saveAs")}</h3>
            <ClearableTextInput
              autoFocus
              value={saveName}
              onValueChange={setSaveName}
              placeholder={t("scene.newName")}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSave();
                if (e.key === "Escape") setSaving(false);
              }}
            />
            <div className="modal-actions">
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleSave}
              >
                {t("scene.save")}
              </button>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => setSaving(false)}
              >
                {t("app.cancel")}
              </button>
            </div>
          </div>
        </div>
      )}

      {pendingDelete && (
        <div
          className="modal-overlay modal-overlay-front"
          onClick={() => setPendingDelete("")}
        >
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t("scene.delete")}</h3>
            <div className="scene-confirm-text">
              {t("scene.confirmDelete", { name: pendingDelete })}
            </div>
            <div className="modal-actions">
              <button
                type="button"
                className="btn btn-danger"
                onClick={() => void handleDelete()}
              >
                {t("scene.delete")}
              </button>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => setPendingDelete("")}
              >
                {t("app.cancel")}
              </button>
            </div>
          </div>
        </div>
      )}

      {open && (
        <div className="modal-overlay" onClick={closeSceneModal}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t("scene.label")}</h3>
            {running && (
              <div className="scene-hint">{t("scene.runningLocked")}</div>
            )}
            {scenes.length === 0 ? (
              <div className="scene-empty">{t("scene.empty")}</div>
            ) : (
              <ul className="scene-list">
                {scenes.map((name) => (
                  <li
                    key={name}
                    className={`scene-item${selected === name ? " scene-item-active" : ""}`}
                  >
                    {renaming === name && !running ? (
                      <>
                        <ClearableTextInput
                          autoFocus
                          value={newName}
                          onValueChange={setNewName}
                          containerClassName="scene-rename-input"
                          onKeyDown={(e) => {
                            if (e.key === "Enter") void handleRename(name);
                            if (e.key === "Escape") {
                              setRenaming("");
                              setNewName("");
                            }
                          }}
                        />
                        <div className="scene-actions">
                          <button
                            type="button"
                            className="btn-sm"
                            onClick={() => void handleRename(name)}
                          >
                            {t("app.confirm")}
                          </button>
                          <button
                            type="button"
                            className="btn-sm btn-danger"
                            onClick={() => {
                              setRenaming("");
                              setNewName("");
                            }}
                          >
                            {t("app.cancel")}
                          </button>
                        </div>
                      </>
                    ) : (
                      <button
                        type="button"
                        className="scene-name-btn"
                        disabled={running}
                        onClick={() => handleLoad(name)}
                      >
                        {name}
                      </button>
                    )}
                    <div className="scene-actions">
                      <button
                        type="button"
                        className="btn-sm"
                        onClick={() => handleExport(name)}
                      >
                        {t("scene.export")}
                      </button>
                      <button
                        type="button"
                        className="btn-sm"
                        disabled={running}
                        onClick={() => {
                          setRenaming(name);
                          setNewName(name);
                        }}
                      >
                        {t("scene.rename")}
                      </button>
                      <button
                        type="button"
                        className="btn-sm btn-danger"
                        disabled={running}
                        onClick={() => setPendingDelete(name)}
                      >
                        {t("scene.delete")}
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
            <div className="scene-modal-actions">
              <button
                type="button"
                className="btn btn-secondary"
                disabled={running}
                onClick={() => void handleImport()}
              >
                {t("scene.import")}
              </button>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={closeSceneModal}
              >
                {t("app.close")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
