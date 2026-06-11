import { useEffect, useRef, useState, type FocusEvent, type InputHTMLAttributes } from 'react';
import i18n from '../i18n';

function joinClassNames(...classes: Array<string | undefined>) {
  return classes.filter(Boolean).join(' ');
}

function ClearButton({
  visible,
  label,
  onClick,
}: {
  visible: boolean;
  label: string;
  onClick: () => void;
}) {
  if (!visible) return null;

  return (
    <button
      type="button"
      className="input-clear-btn"
      aria-label={label}
      title={label}
      onMouseDown={e => e.preventDefault()}
      onClick={onClick}
    >
      ×
    </button>
  );
}

interface ClearableTextInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange'> {
  value: string;
  onValueChange: (value: string) => void;
  containerClassName?: string;
  clearLabel?: string;
}

export function ClearableTextInput({
  value,
  onValueChange,
  className,
  containerClassName,
  clearLabel,
  disabled,
  ...props
}: ClearableTextInputProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const resolvedClearLabel = clearLabel ?? i18n.t('app.clearInput');

  return (
    <div className={joinClassNames('clearable-input', containerClassName)}>
      <input
        {...props}
        ref={inputRef}
        className={className}
        value={value}
        disabled={disabled}
        onChange={e => onValueChange(e.target.value)}
      />
      <ClearButton
        visible={!disabled && value !== ''}
        label={resolvedClearLabel}
        onClick={() => {
          onValueChange('');
          inputRef.current?.focus();
        }}
      />
    </div>
  );
}

interface ClearableNumberInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'value' | 'onChange'> {
  value?: number | null;
  onValueChange: (value: number) => void;
  containerClassName?: string;
  clearLabel?: string;
}

function toDraftValue(value?: number | null) {
  if (value === null || value === undefined || Number.isNaN(value)) return '';
  return String(value);
}

export function ClearableNumberInput({
  value,
  onValueChange,
  className,
  containerClassName,
  clearLabel,
  disabled,
  onFocus,
  onBlur,
  ...props
}: ClearableNumberInputProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const isFocusedRef = useRef(false);
  const [draft, setDraft] = useState(() => toDraftValue(value));
  const resolvedClearLabel = clearLabel ?? i18n.t('app.clearInput');

  useEffect(() => {
    if (!isFocusedRef.current) {
      setDraft(toDraftValue(value));
    }
  }, [value]);

  const handleFocus = (event: FocusEvent<HTMLInputElement>) => {
    isFocusedRef.current = true;
    onFocus?.(event);
  };

  const handleBlur = (event: FocusEvent<HTMLInputElement>) => {
    isFocusedRef.current = false;

    if (draft === '') {
      setDraft(toDraftValue(value));
      onBlur?.(event);
      return;
    }

    const parsed = Number(draft);
    if (Number.isNaN(parsed)) {
      setDraft(toDraftValue(value));
      onBlur?.(event);
      return;
    }

    onValueChange(parsed);
    setDraft(String(parsed));
    onBlur?.(event);
  };

  return (
    <div className={joinClassNames('clearable-input', containerClassName)}>
      <input
        {...props}
        ref={inputRef}
        type="number"
        className={className}
        value={draft}
        disabled={disabled}
        onFocus={handleFocus}
        onBlur={handleBlur}
        onChange={e => {
          const next = e.target.value;
          setDraft(next);
          if (next === '') return;

          const parsed = Number(next);
          if (!Number.isNaN(parsed)) {
            onValueChange(parsed);
          }
        }}
      />
      <ClearButton
        visible={!disabled && draft !== ''}
        label={resolvedClearLabel}
        onClick={() => {
          setDraft('');
          inputRef.current?.focus();
        }}
      />
    </div>
  );
}
