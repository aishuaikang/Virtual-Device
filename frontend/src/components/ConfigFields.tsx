import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ClearableNumberInput, ClearableTextInput } from "./ClearableInput";

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  );
}

export function NumberField({
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
  disabled,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number | "any";
  disabled?: boolean;
}) {
  return (
    <Field label={label}>
      <ClearableNumberInput
        aria-label={label}
        value={value}
        onValueChange={onChange}
        min={min}
        max={max}
        step={step}
        disabled={disabled}
      />
    </Field>
  );
}

export function TextField({
  label,
  value,
  onChange,
  disabled,
  list,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  list?: string;
  placeholder?: string;
}) {
  return (
    <Field label={label}>
      <ClearableTextInput
        aria-label={label}
        value={value}
        onValueChange={onChange}
        disabled={disabled}
        list={list}
        placeholder={placeholder}
        spellCheck={false}
      />
    </Field>
  );
}

export function HostsField({
  value,
  onChange,
  disabled,
}: {
  value: string[];
  onChange: (value: string[]) => void;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const text = value.join(", ");
  const [draft, setDraft] = useState(text);
  const focused = useRef(false);
  useEffect(() => {
    if (!focused.current) setDraft(text);
  }, [text]);
  return (
    <Field label={t("config.servers")}>
      <ClearableTextInput
        value={draft}
        aria-label={t("config.servers")}
        onValueChange={(next) => {
          setDraft(next);
          onChange(next.split(/[,，\s]+/).filter(Boolean));
        }}
        disabled={disabled}
        placeholder="192.168.1.10, 192.168.1.11"
        spellCheck={false}
        onFocus={() => {
          focused.current = true;
        }}
        onBlur={() => {
          focused.current = false;
          setDraft(text);
        }}
      />
    </Field>
  );
}

export function Toggle({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <label className="switch" title={label}>
      <input
        type="checkbox"
        role="switch"
        aria-label={label}
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        disabled={disabled}
      />
      <span aria-hidden="true" />
    </label>
  );
}

export function Advanced({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <details className="advanced">
      <summary>{t("config.advanced")}</summary>
      <div className="form-grid">{children}</div>
    </details>
  );
}
