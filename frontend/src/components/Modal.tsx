import { Modal as AntModal } from "antd";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

export default function Modal({ title, onClose, children, wide = false }: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <AntModal open centered title={title} onCancel={onClose} footer={null}
      width={wide ? 700 : 460} className="simulator-modal"
      closable={{ "aria-label": t("app.close") }}
      styles={{
        content: { padding: 0, maxHeight: "calc(100dvh - 32px)", display: "flex", flexDirection: "column", overflow: "hidden" },
        header: { padding: "12px 16px", margin: 0, borderBottom: "1px solid var(--line)" },
        body: { overflowY: "auto" },
      }}>
      {children}
    </AntModal>
  );
}
