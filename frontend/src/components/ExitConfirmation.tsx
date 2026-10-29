import { Button } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ConfirmClose } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import ErrorMessage from "./ErrorMessage";
import Modal from "./Modal";

export default function ExitConfirmation({ beforeSave }: { beforeSave: () => Promise<unknown> }) {
  const { t } = useTranslation();
  const [scene, setScene] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const pending = useRef(false);
  useEffect(() => EventsOn("close-requested", (name: string) => {
    setScene(name);
    setError(null);
  }), []);

  const close = () => { if (!pending.current) setScene(null); };
  const confirm = async (save: boolean) => {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError(null);
    try {
      if (save) await beforeSave();
      await ConfirmClose(save);
      setScene(null);
    } catch (cause) {
      setError(cause);
    } finally {
      pending.current = false;
      setBusy(false);
    }
  };

  if (scene === null) return null;
  return (
    <Modal title={t("scene.exitTitle")} onClose={close}>
      <div className="dialog-body">
        <p>{scene ? t("scene.exitExisting", { name: scene }) : t("scene.exitNew")}</p>
        {error != null && <div className="inline-error" role="alert"><ErrorMessage error={error} messageKey="errors.scene" /></div>}
      </div>
      <footer className="dialog-actions">
        <Button disabled={busy} onClick={close}>{t("app.cancel")}</Button>
        <Button danger disabled={busy} onClick={() => void confirm(false)}>{t("scene.discardExit")}</Button>
        <Button type="primary" disabled={busy} onClick={() => void confirm(true)}>{t("scene.saveExit")}</Button>
      </footer>
    </Modal>
  );
}
