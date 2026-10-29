import { Input, InputNumber, Button, type InputProps, type InputNumberProps } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

interface ClearableTextInputProps extends Omit<InputProps, "value" | "onChange"> {
  value: string;
  onValueChange: (value: string) => void;
  containerClassName?: string;
  clearLabel?: string;
}

export function ClearableTextInput({
  value, onValueChange, className, containerClassName, clearLabel, ...props
}: ClearableTextInputProps) {
  const { t } = useTranslation();
  return (
    <Input
      {...props}
      className={["clearable-input", containerClassName, className].filter(Boolean).join(" ")}
      value={value}
      onChange={(event) => onValueChange(event.target.value)}
      allowClear={{ clearIcon: <span aria-label={clearLabel ?? t("app.clearInput")}>×</span> }}
    />
  );
}

interface ClearableNumberInputProps extends Omit<InputNumberProps<number>, "value" | "onChange" | "step"> {
  value?: number | null;
  onValueChange: (value: number) => void;
  step?: number | "any";
  containerClassName?: string;
  clearLabel?: string;
}

function numberOrNull(value?: number | null) {
  return value != null && Number.isFinite(value) ? value : null;
}

export function ClearableNumberInput({
  value, onValueChange, className, containerClassName, clearLabel, disabled,
  onFocus, onBlur, step, ...props
}: ClearableNumberInputProps) {
  const { t } = useTranslation();
  const inputRef = useRef<{ focus: () => void } | null>(null);
  const focused = useRef(false);
  const [draft, setDraft] = useState(() => numberOrNull(value));
  useEffect(() => {
    if (!focused.current) setDraft(numberOrNull(value));
  }, [value]);

  return (
    <InputNumber<number>
      {...props}
      ref={(input) => { inputRef.current = input; }}
      className={["clearable-input", containerClassName, className].filter(Boolean).join(" ")}
      value={draft}
      disabled={disabled}
      step={step === "any" ? undefined : step}
      changeOnBlur={false}
      onFocus={(event) => {
        focused.current = true;
        onFocus?.(event);
      }}
      onBlur={(event) => {
        focused.current = false;
        if (draft === null) setDraft(numberOrNull(value));
        onBlur?.(event);
      }}
      onChange={(next) => {
        setDraft(next);
        if (next !== null && Number.isFinite(next)) onValueChange(next);
      }}
      suffix={!disabled && draft !== null ? (
        <Button
          type="text"
          className="number-clear"
          aria-label={clearLabel ?? t("app.clearInput")}
          title={clearLabel ?? t("app.clearInput")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => {
            setDraft(null);
            inputRef.current?.focus();
          }}
        >×</Button>
      ) : undefined}
    />
  );
}
