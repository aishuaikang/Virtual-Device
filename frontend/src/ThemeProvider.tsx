import { ConfigProvider } from "antd";
import zh from "antd/locale/zh_CN";
import en from "antd/locale/en_US";
import tr from "antd/locale/tr_TR";
import fr from "antd/locale/fr_FR";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { getCurrentLanguage } from "./i18n";

const locales = { zh, en, tr, fr };

export default function ThemeProvider({ children }: { children: ReactNode }) {
  useTranslation();
  return (
    <ConfigProvider componentSize="small" locale={locales[getCurrentLanguage()]} theme={{
      token: {
        colorPrimary: "#3567e8", colorInfo: "#3567e8",
        colorText: "#1c2940", colorTextSecondary: "#637289",
        colorBorder: "#dce2ec", colorBgLayout: "#f4f6fa",
        borderRadius: 6, fontSize: 13, controlHeight: 30, controlHeightSM: 28,
        fontFamily: '"Avenir Next", "Segoe UI Variable Text", "PingFang SC", "Microsoft YaHei", sans-serif',
      },
      components: {
        Button: { primaryShadow: "0 2px 4px #3567e820", defaultShadow: "0 1px 2px #162b4810" },
      },
    }}>
      {children}
    </ConfigProvider>
  );
}
