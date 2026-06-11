import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ClearableTextInput } from "./ClearableInput";

const TIMESTAMP_RE = /^\d{2}:\d{2}:\d{2}\.\d{6}$/;
const IP_RE = /^(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?$/;
const HEX_RE = /^0x[0-9A-Fa-f]+$/;
const LOG_TOKEN_RE =
  /\d{2}:\d{2}:\d{2}\.\d{6}|\[[^\]]+\]|\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b|0x[0-9A-Fa-f]+/g;

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function lineClass(line: string) {
  if (
    line.includes("[ERROR]") ||
    line.includes("ERROR") ||
    line.includes("失败") ||
    line.includes("错误") ||
    line.includes("异常") ||
    line.includes("超时")
  )
    return "log-line log-line-error";
  if (line.includes("[WARN]") || line.includes("WARN") || line.includes("警告"))
    return "log-line log-line-warn";
  if (
    line.includes("[INFO]") ||
    line.includes("INFO") ||
    line.includes("成功") ||
    line.includes("启动") ||
    line.includes("停止")
  )
    return "log-line log-line-info";
  return "log-line";
}

function tokenClass(token: string) {
  if (TIMESTAMP_RE.test(token)) return "log-token-time";
  if (IP_RE.test(token)) return "log-token-addr";
  if (HEX_RE.test(token)) return "log-token-hex";
  if (token.startsWith("[") && token.endsWith("]")) {
    const upper = token.toUpperCase();
    if (
      upper.includes("ERROR") ||
      token.includes("失败") ||
      token.includes("错误") ||
      token.includes("异常")
    ) {
      return "log-token-level-error";
    }
    if (upper.includes("WARN") || token.includes("警告")) {
      return "log-token-level-warn";
    }
    if (upper.includes("INFO")) {
      return "log-token-level-info";
    }
    return "log-token-tag";
  }
  return "";
}

function highlightText(
  text: string,
  filter: string,
  keyPrefix: string,
): ReactNode[] {
  if (!filter) return [text];

  const matches = [...text.matchAll(new RegExp(escapeRegExp(filter), "gi"))];
  if (matches.length === 0) return [text];

  const nodes: ReactNode[] = [];
  let cursor = 0;

  matches.forEach((match, index) => {
    const start = match.index ?? 0;
    const end = start + match[0].length;

    if (start > cursor) {
      nodes.push(text.slice(cursor, start));
    }
    nodes.push(
      <mark key={`${keyPrefix}-${index}`} className="log-highlight">
        {text.slice(start, end)}
      </mark>,
    );
    cursor = end;
  });

  if (cursor < text.length) {
    nodes.push(text.slice(cursor));
  }

  return nodes;
}

function renderLine(line: string, filter: string) {
  const tokens: ReactNode[] = [];
  let lastIndex = 0;

  for (const match of line.matchAll(LOG_TOKEN_RE)) {
    const token = match[0];
    const index = match.index ?? 0;

    if (index > lastIndex) {
      tokens.push(
        ...highlightText(
          line.slice(lastIndex, index),
          filter,
          `plain-${lastIndex}`,
        ),
      );
    }

    const className = tokenClass(token);
    const content = highlightText(token, filter, `token-${index}`);
    tokens.push(
      className ? (
        <span key={`token-${index}`} className={className}>
          {content}
        </span>
      ) : (
        <span key={`token-${index}`}>{content}</span>
      ),
    );

    lastIndex = index + token.length;
  }

  if (lastIndex < line.length) {
    tokens.push(
      ...highlightText(line.slice(lastIndex), filter, `plain-${lastIndex}`),
    );
  }

  return tokens;
}

function formatExportTimestamp(date: Date) {
  return date
    .toISOString()
    .replace(/\.\d{3}Z$/, "")
    .replace("T", "_")
    .replace(/:/g, "-");
}

export default function LogViewer() {
  const { t } = useTranslation();
  const [lines, setLines] = useState<string[]>([]);
  const [filter, setFilter] = useState("");
  const [atBottom, setAtBottom] = useState(true);
  const bodyRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const userScrolled = useRef(false);

  useEffect(() => {
    const cancel = EventsOn("log", (line: string) => {
      setLines((prev) => {
        const next = [...prev, line];
        return next.length > 2000 ? next.slice(-2000) : next;
      });
    });
    return cancel;
  }, []);

  useEffect(() => {
    if (!userScrolled.current) {
      const container = bodyRef.current;
      if (container) {
        container.scrollTop = container.scrollHeight;
      }
    }
  }, [lines]);

  useEffect(() => {
    const el = bodyRef.current;
    if (!el) return;
    const onScroll = () => {
      const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
      userScrolled.current = !isAtBottom;
      setAtBottom(isAtBottom);
    };
    el.addEventListener("scroll", onScroll);
    return () => el.removeEventListener("scroll", onScroll);
  }, []);

  const handleClear = () => setLines([]);

  const handleExport = () => {
    const blob = new Blob([lines.join("\n")], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `vd-log-${formatExportTimestamp(new Date())}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const scrollToBottom = () => {
    userScrolled.current = false;
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const filtered = filter
    ? lines.filter((l) => l.toLowerCase().includes(filter.toLowerCase()))
    : lines;

  return (
    <div className="log-viewer">
      <div className="log-header">
        <span className="panel-title">{t("log.title")}</span>
        <ClearableTextInput
          containerClassName="log-filter-wrap"
          className="log-filter"
          value={filter}
          onValueChange={setFilter}
          placeholder={t("log.filter")}
        />
        <div className="log-actions">
          <button onClick={handleExport} className="btn-sm">
            {t("log.export")}
          </button>
          <button onClick={handleClear} className="btn-sm btn-danger">
            {t("log.clear")}
          </button>
        </div>
      </div>
      <div className="log-container">
        <div className="log-body" ref={bodyRef}>
          {filtered.length === 0 ? (
            <span className="log-placeholder">
              {filter ? t("log.noMatch") : t("log.placeholder")}
            </span>
          ) : (
            filtered.map((line, i) => (
              <div key={i} className={lineClass(line)}>
                {renderLine(line, filter)}
              </div>
            ))
          )}
          <div ref={bottomRef} />
        </div>
        {!atBottom && (
          <button className="log-scroll-btn" onClick={scrollToBottom}>
            {t("log.scrollBottom")}
          </button>
        )}
      </div>
    </div>
  );
}
