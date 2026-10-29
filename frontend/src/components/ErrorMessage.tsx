import { useTranslation } from "react-i18next";
import { Collapse } from "antd";
import { errorReason, errorText } from "../i18n/errors";

export default function ErrorMessage({ error, messageKey = "errors.operation" }: { error: unknown; messageKey?: string }) {
  const { t } = useTranslation();
  const reason = errorReason(error);
  return (
    <div className="localized-error">
      <span>{t(messageKey)}{reason ? ` ${t(reason)}` : ""}</span>
      <Collapse ghost size="small" items={[{
        key: "details", label: t("app.details"), children: <pre>{errorText(error)}</pre>,
      }]} />
    </div>
  );
}
