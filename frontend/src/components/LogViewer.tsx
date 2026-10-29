import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Checkbox } from "antd";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ClearableTextInput } from "./ClearableInput";
import Icon from "./Icon";

export default function LogViewer() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [lines, setLines] = useState<string[]>([]);
  const [filter, setFilter] = useState("");
  const [follow, setFollow] = useState(true);
  const pending = useRef<string[]>([]);
  const body = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const cancel = EventsOn("log", (line: string) => {
      pending.current.push(line);
      if (pending.current.length > 2000)
        pending.current.splice(0, pending.current.length - 2000);
    });
    const timer = window.setInterval(() => {
      if (!pending.current.length) return;
      const next = pending.current;
      pending.current = [];
      setLines((current) => [...current, ...next].slice(-2000));
    }, 150);
    return () => {
      cancel();
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    if (open && follow && body.current)
      body.current.scrollTop = body.current.scrollHeight;
  }, [open, follow, lines, filter]);

  const exportLogs = () => {
    const url = URL.createObjectURL(
      new Blob([lines.join("\n")], { type: "text/plain;charset=utf-8" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = `simulator-${new Date().toISOString().replace(/[:.]/g, "-")}.log`;
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  };
  const filtered = filter
    ? lines.filter((line) => line.toLowerCase().includes(filter.toLowerCase()))
    : lines;

  return (
    <section className={`log-drawer${open ? " is-open" : ""}`}>
      <div className="log-toolbar">
        <Button type="text"
          className="log-toggle"
          onClick={() => setOpen(!open)}
          aria-expanded={open}
          aria-controls="log-output"
        >
          <Icon name="log" />
          {t("log.title")}
          <span className="count">{lines.length}</span>
          <Icon className="log-chevron" name="chevron" />
        </Button>
        {open && (
          <>
            <ClearableTextInput
              containerClassName="log-filter"
              aria-label={t("log.filter")}
              placeholder={t("log.filter")}
              value={filter}
              onValueChange={setFilter}
            />
            <Checkbox className="log-follow"
                checked={follow}
                onChange={(event) => setFollow(event.target.checked)}
              >
              {t("log.follow")}
            </Checkbox>
            <Button type="text"
              className="icon-button"
              onClick={exportLogs}
              disabled={!lines.length}
              title={t("log.export")}
              aria-label={t("log.export")}
            >
              <Icon name="download" />
            </Button>
            <Button type="text"
              className="icon-button"
              title={t("app.clear")}
              aria-label={t("log.clear")}
              onClick={() => {
                pending.current = [];
                setLines([]);
              }}
              disabled={!lines.length}
            >
              <Icon name="trash" />
            </Button>
          </>
        )}
      </div>
      {open && (
        <div className="log-output" id="log-output" ref={body}>
          {filtered.map((line, index) => (
            <div
              key={index}
              className={
                /ERROR|失败|错误|异常|超时/.test(line)
                  ? "log-line log-error"
                  : /WARN|警告/.test(line)
                    ? "log-line log-warn"
                    : "log-line"
              }
            >
              {line}
            </div>
          ))}
          {filtered.length === 0 && (
            <span className="log-empty">
              {t(filter ? "log.noMatch" : "log.empty")}
            </span>
          )}
        </div>
      )}
    </section>
  );
}
