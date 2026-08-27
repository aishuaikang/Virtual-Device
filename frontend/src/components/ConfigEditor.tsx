import { useState, useEffect, useRef } from "react";
import MapPicker from "./MapPicker";
import { useTranslation } from "react-i18next";
import { config } from "../../wailsjs/go/models";
import {
  GetLocalIPs,
  SelectFile,
} from "../../wailsjs/go/main/App";
import { ClearableNumberInput, ClearableTextInput } from "./ClearableInput";

interface Props {
  cfg: config.Config;
  onChange: (cfg: config.Config) => void;
  disabled?: boolean;
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(true);
  return (
    <div className="config-section">
      <button
        type="button"
        className="section-toggle"
        onClick={() => setOpen((o) => !o)}
      >
        <span>{open ? "▼" : "▶"}</span> {title}
      </button>
      {open && <div className="section-body">{children}</div>}
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="config-field">
      <span className="field-label">{label}</span>
      {children}
    </label>
  );
}

function updateValueAtPath(
  source: Record<string, unknown>,
  path: string[],
  value: unknown,
): Record<string, unknown> {
  const [head, ...rest] = path;
  if (!head) {
    return source;
  }

  return {
    ...source,
    [head]:
      rest.length === 0
        ? value
        : updateValueAtPath(
            ((source[head] as Record<string, unknown> | undefined) ?? {}) as Record<
              string,
              unknown
            >,
            rest,
            value,
          ),
  };
}

function CoordinatePair({
  label,
  lat,
  lng,
  onLatChange,
  onLngChange,
  disabled,
  latLabel,
  lngLabel,
}: {
  label: string;
  lat: number;
  lng: number;
  onLatChange: (value: number) => void;
  onLngChange: (value: number) => void;
  disabled?: boolean;
  latLabel: string;
  lngLabel: string;
}) {
  return (
    <div className="config-field">
      <span className="field-label">{label}</span>
      <div className="field-row">
        <ClearableNumberInput
          step="0.000001"
          placeholder={latLabel}
          value={lat}
          onValueChange={onLatChange}
          disabled={disabled}
        />
        <ClearableNumberInput
          step="0.000001"
          placeholder={lngLabel}
          value={lng}
          onValueChange={onLngChange}
          disabled={disabled}
        />
      </div>
    </div>
  );
}


function ToggleField({
  label,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <label className={`config-toggle${disabled ? " config-toggle-disabled" : ""}`}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>{label}</span>
    </label>
  );
}

function PathSelector({
  value,
  placeholder,
  chooseLabel,
  clearLabel,
  onChoose,
  onClear,
  disabled,
}: {
  value: string;
  placeholder: string;
  chooseLabel: string;
  clearLabel: string;
  onChoose: () => Promise<void>;
  onClear: () => void;
  disabled?: boolean;
}) {
  const shownValue = value || placeholder;

  return (
    <div className="path-picker">
      <button
        type="button"
        className={`path-picker-display${value ? "" : " path-picker-display-empty"}`}
        title={shownValue}
        disabled={disabled}
        onClick={() => void onChoose()}
      >
        <span
          className={value ? "path-picker-value" : "path-picker-placeholder"}
        >
          {shownValue}
        </span>
      </button>
      <div className="path-picker-actions">
        <button
          type="button"
          className="btn-sm"
          disabled={disabled}
          onClick={() => void onChoose()}
        >
          {chooseLabel}
        </button>
        <button
          type="button"
          className="btn-sm btn-danger"
          disabled={disabled || value === ""}
          onClick={onClear}
        >
          {clearLabel}
        </button>
      </div>
    </div>
  );
}

function IPInput({
  value,
  onChange,
  localIPs,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  localIPs: string[];
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);
  return (
    <div className="ip-input-wrap" ref={ref}>
      <ClearableTextInput
        value={value}
        onValueChange={(next) => {
          onChange(next);
          if (next === "") setOpen(false);
        }}
        onFocus={() => !disabled && localIPs.length > 0 && setOpen(true)}
        placeholder="0.0.0.0"
        disabled={disabled}
      />
      {open && localIPs.length > 0 && (
        <ul className="ip-dropdown">
          {localIPs.map((ip) => (
            <li
              key={ip}
              onMouseDown={() => {
                onChange(ip);
                setOpen(false);
              }}
            >
              {ip}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function HostsInput({
  value,
  onChange,
  localIPs,
  disabled,
}: {
  value: string[];
  onChange: (v: string[]) => void;
  localIPs: string[];
  disabled?: boolean;
}) {
  const raw = value.join(", ");
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);
  return (
    <div className="ip-input-wrap" ref={ref}>
      <ClearableTextInput
        value={raw}
        onValueChange={(next) => {
          onChange(
            next
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
          );
          if (next.trim() === "") setOpen(false);
        }}
        onFocus={() => !disabled && localIPs.length > 0 && setOpen(true)}
        placeholder="0.0.0.0"
        disabled={disabled}
      />
      {open && localIPs.length > 0 && (
        <ul className="ip-dropdown">
          {localIPs.map((ip) => (
            <li
              key={ip}
              onMouseDown={() => {
                onChange([ip]);
                setOpen(false);
              }}
            >
              {ip}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function DroneCard({
  drone,
  index,
  onDelete,
  onChange,
  onCoordinateChange,
  t,
  disabled,
}: {
  drone: config.PredefinedDrone;
  index: number;
  onDelete: () => void;
  onChange: (field: string, value: unknown) => void;
  onCoordinateChange: (
    key: "drone_gps" | "pilot_gps",
    axis: "lat" | "lng",
    value: number,
  ) => void;
  t: (k: string) => string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const label = drone.model || drone.serial || `#${index + 1}`;
  return (
    <div className="drone-card">
      <div className="drone-card-header" onClick={() => setOpen((o) => !o)}>
        <span className="drone-card-title">
          {open ? "▼" : "▶"} {label}
        </span>
        <button
          className="btn-sm btn-danger"
          disabled={disabled}
          onClick={(e) => {
            e.stopPropagation();
            onDelete();
          }}
        >
          {t("config.deleteDrone")}
        </button>
      </div>
      {open && (
        <div className="drone-card-body">
          <div className="field-row">
            <label className="config-field">
              <span className="field-label">{t("config.serial")}</span>
              <ClearableTextInput
                value={drone.serial}
                onValueChange={(value) => onChange("serial", value)}
                disabled={disabled}
              />
            </label>
            <label className="config-field">
              <span className="field-label">{t("config.model")}</span>
              <ClearableTextInput
                value={drone.model}
                onValueChange={(value) => onChange("model", value)}
                disabled={disabled}
              />
            </label>
          </div>
          <div className="field-row">
            <label className="config-field">
              <span className="field-label">{t("config.freq")}</span>
              <ClearableNumberInput
                value={drone.freq}
                onValueChange={(value) => onChange("freq", value)}
                disabled={disabled}
              />
            </label>
            <label className="config-field">
              <span className="field-label">{t("config.rssi")}</span>
              <ClearableNumberInput
                value={drone.rssi}
                onValueChange={(value) => onChange("rssi", value)}
                disabled={disabled}
              />
            </label>
          </div>
          <label className="config-field">
            <span className="field-label">{t("config.type")}</span>
            <select
              value={drone.type}
              disabled={disabled}
              onChange={(e) => onChange("type", e.target.value)}
            >
              <option value="AUTO">AUTO</option>
              <option value="DID">DID</option>
              <option value="RID">RID</option>
            </select>
          </label>
          <CoordinatePair
            label={t("config.droneGps")}
            lat={drone.drone_gps?.lat ?? 0}
            lng={drone.drone_gps?.lng ?? 0}
            onLatChange={(value) => onCoordinateChange("drone_gps", "lat", value)}
            onLngChange={(value) => onCoordinateChange("drone_gps", "lng", value)}
            disabled={disabled}
            latLabel={t("config.lat")}
            lngLabel={t("config.lng")}
          />
          <CoordinatePair
            label={t("config.pilotGps")}
            lat={drone.pilot_gps?.lat ?? 0}
            lng={drone.pilot_gps?.lng ?? 0}
            onLatChange={(value) => onCoordinateChange("pilot_gps", "lat", value)}
            onLngChange={(value) => onCoordinateChange("pilot_gps", "lng", value)}
            disabled={disabled}
            latLabel={t("config.lat")}
            lngLabel={t("config.lng")}
          />
        </div>
      )}
    </div>
  );
}

function DetectionCard({
  detection,
  index,
  count,
  onDelete,
  onChange,
  localIPs,
  t,
  disabled,
}: {
  detection: config.DetectionConfig;
  index: number;
  count: number;
  onDelete: () => void;
  onChange: (field: keyof config.DetectionConfig, value: unknown) => void;
  localIPs: string[];
  t: (k: string, options?: Record<string, unknown>) => string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(index === 0);
  const label = t("config.detectionIndex", { index: index + 1 });
  const subtitle = `${detection.deviceID ?? "-"} / ${detection.host ?? "-"}:${detection.port ?? "-"}`;

  return (
    <div className="drone-card">
      <div className="drone-card-header" onClick={() => setOpen((o) => !o)}>
        <span className="drone-card-title">
          {open ? "▼" : "▶"} {label} · {subtitle}
        </span>
        <button
          className="btn-sm btn-danger"
          disabled={disabled || count <= 1}
          onClick={(e) => {
            e.stopPropagation();
            onDelete();
          }}
        >
          {t("config.deleteDetection")}
        </button>
      </div>
      {open && (
        <div className="drone-card-body">
          <ToggleField
            label={t("config.enabled")}
            checked={detection.enabled !== false}
            onChange={(checked) => onChange("enabled", checked)}
            disabled={disabled}
          />
          <div className="field-row">
            <Field label={t("config.deviceID")}>
              <ClearableNumberInput
                value={detection.deviceID}
                onValueChange={(value) => onChange("deviceID", value)}
                disabled={disabled}
              />
            </Field>
            <Field label={t("config.droneCount")}>
              <ClearableNumberInput
                value={detection.drone_count}
                onValueChange={(value) => onChange("drone_count", value)}
                min={0}
                disabled={disabled}
              />
            </Field>
          </div>
          <div className="field-row">
            <Field label={t("config.host")}>
              <IPInput
                value={detection.host ?? ""}
                onChange={(value) => onChange("host", value)}
                localIPs={localIPs}
                disabled={disabled}
              />
            </Field>
            <Field label={t("config.port")}>
              <ClearableNumberInput
                value={detection.port}
                onValueChange={(value) => onChange("port", value)}
                disabled={disabled}
              />
            </Field>
          </div>
          <Field label={t("config.heartbeatInterval")}>
            <ClearableNumberInput
              value={detection.heartbeat_interval}
              onValueChange={(value) => onChange("heartbeat_interval", value)}
              disabled={disabled}
            />
          </Field>
        </div>
      )}
    </div>
  );
}

export default function ConfigEditor({ cfg, onChange, disabled }: Props) {
  const { t } = useTranslation();
  const [localIPs, setLocalIPs] = useState<string[]>([]);
  const [showMap, setShowMap] = useState(false);
  const detections =
    cfg.detections && cfg.detections.length > 0
      ? cfg.detections
      : cfg.detection
        ? [cfg.detection]
        : [];

  useEffect(() => {
    GetLocalIPs().then((ips) =>
      setLocalIPs([
        "0.0.0.0",
        "127.0.0.1",
        ...[...(ips ?? [])].filter(
          (ip, index, items) =>
            ip !== "0.0.0.0" &&
            ip !== "127.0.0.1" &&
            items.indexOf(ip) === index,
        ),
      ]),
    );
  }, []);

  const set = (path: string[], value: unknown) => {
    onChange(
      updateValueAtPath(
        cfg as unknown as Record<string, unknown>,
        path,
        value,
      ) as unknown as config.Config,
    );
  };

  const num = (path: string[]) => (value: number) => set(path, value);

  const setDetections = (next: config.DetectionConfig[]) => {
    onChange({
      ...cfg,
      detections: next,
      detection: next[0] ?? cfg.detection,
    } as config.Config);
  };

  const setDetection = (
    index: number,
    field: keyof config.DetectionConfig,
    value: unknown,
  ) => {
    const next = detections.map((item, i) =>
      i === index ? ({ ...item, [field]: value } as config.DetectionConfig) : item,
    );
    setDetections(next);
  };

  const addDetection = () => {
    const last = detections[detections.length - 1] ?? cfg.detection;
    const detection = {
      enabled: true,
      deviceID: (last?.deviceID ?? 1999) + 1,
      drone_count: last?.drone_count ?? 1,
      host: last?.host || "0.0.0.0",
      port: (last?.port ?? 9024) + 2,
      heartbeat_interval: last?.heartbeat_interval ?? 5,
    } as config.DetectionConfig;
    setDetections([...detections, detection]);
  };

  const deleteDetection = (index: number) => {
    if (detections.length <= 1) {
      return;
    }
    const next = [...detections];
    next.splice(index, 1);
    setDetections(next);
  };

  const pickFile = async (
    currentPath: string,
    title: string,
    path: string[],
  ) => {
    try {
      const selected = await SelectFile(currentPath, title);
      if (selected) {
        set(path, selected);
      }
    } catch (error) {
      console.error("Failed to select file:", error);
    }
  };

  const addDrone = () => {
    const drone = {
      serial: "",
      model: "",
      freq: 2437,
      rssi: -70,
      drone_gps: { lat: cfg.center_point.lat, lng: cfg.center_point.lng },
      pilot_gps: { lat: 0, lng: 0 },
      type: "AUTO",
    } as unknown as config.PredefinedDrone;
    set(["predefined_drones"], [...(cfg.predefined_drones ?? []), drone]);
  };

  const delDrone = (i: number) => {
    const next = [...(cfg.predefined_drones ?? [])];
    next.splice(i, 1);
    set(["predefined_drones"], next);
  };

  const setDrone = (i: number, field: string, value: unknown) => {
    const next = [...(cfg.predefined_drones ?? [])];
    (next[i] as unknown as Record<string, unknown>)[field] = value;
    set(["predefined_drones"], next);
  };

  const setDroneCoordinate = (
    i: number,
    key: "drone_gps" | "pilot_gps",
    axis: "lat" | "lng",
    value: number,
  ) => {
    const next = [...(cfg.predefined_drones ?? [])];
    const current = next[i];
    if (!current) {
      return;
    }
    next[i] = {
      ...current,
      [key]: {
        ...(current[key] ?? { lat: 0, lng: 0 }),
        [axis]: value,
      },
    } as config.PredefinedDrone;
    set(["predefined_drones"], next);
  };

  return (
    <div className={`config-editor${disabled ? " config-disabled" : ""}`}>
      <Section title={t("config.global")}>
        <Field label={t("config.centerPoint")}>
          <div className="coord-field">
            <ClearableNumberInput
              step="0.000001"
              placeholder={t("config.lat")}
              value={cfg.center_point?.lat}
              onValueChange={(lat) =>
                onChange({
                  ...cfg,
                  center_point: { lat, lng: cfg.center_point?.lng ?? 0 },
                } as any)
              }
              disabled={disabled}
            />
            <ClearableNumberInput
              step="0.000001"
              placeholder={t("config.lng")}
              value={cfg.center_point?.lng}
              onValueChange={(lng) =>
                onChange({
                  ...cfg,
                  center_point: { lat: cfg.center_point?.lat ?? 0, lng },
                } as any)
              }
              disabled={disabled}
            />
            <button
              className="btn-map"
              disabled={disabled}
              onClick={() => setShowMap(true)}
            >
              {t("config.map")}
            </button>
          </div>
        </Field>
        <Field label={t("config.refreshInterval")}>
          <ClearableNumberInput
            value={cfg.random_drone_refresh_interval}
            onValueChange={num(["random_drone_refresh_interval"])}
            min={0}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.maxDirectionChange")}>
          <ClearableNumberInput
            value={cfg.max_direction_change}
            onValueChange={num(["max_direction_change"])}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.maxDistance")}>
          <ClearableNumberInput
            value={cfg.max_distance_from_center_point}
            onValueChange={num(["max_distance_from_center_point"])}
            disabled={disabled}
          />
        </Field>
        <div className="field-row">
          <Field label={t("config.minPushSpeed")}>
            <ClearableNumberInput
              value={cfg.min_push_speed}
              onValueChange={num(["min_push_speed"])}
              min={0}
              disabled={disabled}
            />
          </Field>
          <Field label={t("config.maxPushSpeed")}>
            <ClearableNumberInput
              value={cfg.max_push_speed}
              onValueChange={num(["max_push_speed"])}
              min={0}
              disabled={disabled}
            />
          </Field>
        </div>
      </Section>

      <Section title={`${t("config.detections")} (${detections.length})`}>
        {detections.map((detection, index) => (
          <DetectionCard
            key={index}
            detection={detection}
            index={index}
            count={detections.length}
            onDelete={() => deleteDetection(index)}
            onChange={(field, value) => setDetection(index, field, value)}
            localIPs={localIPs}
            t={t}
            disabled={disabled}
          />
        ))}
        <button
          className="btn btn-secondary"
          style={{ margin: "4px 0" }}
          disabled={disabled}
          onClick={addDetection}
        >
          {t("config.addDetection")}
        </button>
      </Section>

      <Section title={t("config.analysis")}>
        <ToggleField
          label={t("config.enabled")}
          checked={cfg.analysis?.enabled !== false}
          onChange={(checked) => set(["analysis", "enabled"], checked)}
          disabled={disabled}
        />
        <Field label={t("config.deviceID")}>
          <ClearableNumberInput
            value={cfg.analysis?.deviceID}
            onValueChange={num(["analysis", "deviceID"])}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.droneCount")}>
          <ClearableNumberInput
            value={cfg.analysis?.drone_count}
            onValueChange={num(["analysis", "drone_count"])}
            min={0}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.hosts")}>
          <HostsInput
            value={cfg.analysis?.hosts ?? []}
            onChange={(v) => set(["analysis", "hosts"], v)}
            localIPs={localIPs}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.port")}>
          <ClearableNumberInput
            value={cfg.analysis?.port}
            onValueChange={num(["analysis", "port"])}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.emptyPacketProb")}>
          <ClearableNumberInput
            value={cfg.analysis?.empty_packet_probability}
            onValueChange={num(["analysis", "empty_packet_probability"])}
            min={0}
            max={100}
            disabled={disabled}
          />
        </Field>
        <Field label={t("config.o3DataFile")}>
          <PathSelector
            value={cfg.analysis?.o3_plus_o4_data_file ?? ""}
            placeholder={t("config.noPathSelected")}
            chooseLabel={t("config.chooseFile")}
            clearLabel={t("app.clear")}
            onChoose={() =>
              pickFile(
                cfg.analysis?.o3_plus_o4_data_file ?? "",
                t("config.o3DataFile"),
                ["analysis", "o3_plus_o4_data_file"],
              )
            }
            onClear={() => set(["analysis", "o3_plus_o4_data_file"], undefined)}
            disabled={disabled}
          />
        </Field>
      </Section>

      <Section title={t("config.directedStrike")}>
        <div className="simulator-section-intro">
          <span className="simulator-section-kicker">TCP DEVICE</span>
          <span>{t("config.directedStrikeHint")}</span>
        </div>
        <ToggleField
          label={t("config.enabled")}
          checked={cfg.directed_strike?.enabled !== false}
          onChange={(checked) =>
            set(["directed_strike", "enabled"], checked)
          }
          disabled={disabled}
        />
        <div className="field-row">
          <Field label={t("config.listenHost")}>
            <IPInput
              value={cfg.directed_strike?.host ?? "0.0.0.0"}
              onChange={(value) => set(["directed_strike", "host"], value)}
              localIPs={localIPs}
              disabled={disabled}
            />
          </Field>
          <Field label={t("config.listenPort")}>
            <ClearableNumberInput
              value={cfg.directed_strike?.port ?? 19000}
              onValueChange={num(["directed_strike", "port"])}
              min={1}
              max={65535}
              disabled={disabled}
            />
          </Field>
        </div>
        <Field label={t("config.responseDelay")}>
          <ClearableNumberInput
            value={cfg.directed_strike?.response_delay_ms ?? 0}
            onValueChange={num(["directed_strike", "response_delay_ms"])}
            min={0}
            max={10000}
            disabled={disabled}
          />
        </Field>
      </Section>

      {(["fpv", "jamming"] as const).map((mod) => (
        <Section key={mod} title={t(`config.${mod}`)}>
          <ToggleField
            label={t("config.enabled")}
            checked={(cfg[mod] as config.BaseConfig)?.enabled !== false}
            onChange={(checked) => set([mod, "enabled"], checked)}
            disabled={disabled}
          />
          <Field label={t("config.deviceID")}>
            <ClearableNumberInput
              value={(cfg[mod] as config.BaseConfig)?.deviceID}
              onValueChange={num([mod, "deviceID"])}
              disabled={disabled}
            />
          </Field>
          <Field label={t("config.hosts")}>
            <HostsInput
              value={(cfg[mod] as config.BaseConfig)?.hosts ?? []}
              onChange={(v) => set([mod, "hosts"], v)}
              localIPs={localIPs}
              disabled={disabled}
            />
          </Field>
          <Field label={t("config.port")}>
            <ClearableNumberInput
              value={(cfg[mod] as config.BaseConfig)?.port}
              onValueChange={num([mod, "port"])}
              disabled={disabled}
            />
          </Field>
        </Section>
      ))}

      <Section
        title={`${t("config.predefinedDrones")} (${(cfg.predefined_drones ?? []).length})`}
      >
        {(cfg.predefined_drones ?? []).map((drone, i) => (
          <DroneCard
            key={i}
            drone={drone}
            index={i}
            onDelete={() => delDrone(i)}
            onChange={(field, value) => setDrone(i, field, value)}
            onCoordinateChange={(key, axis, value) =>
              setDroneCoordinate(i, key, axis, value)
            }
            t={t}
            disabled={disabled}
          />
        ))}
        <button
          className="btn btn-secondary"
          style={{ margin: "4px 0" }}
          disabled={disabled}
          onClick={addDrone}
        >
          {t("config.addDrone")}
        </button>
      </Section>

      {showMap && (
        <MapPicker
          lat={cfg.center_point?.lat ?? 31.2304}
          lng={cfg.center_point?.lng ?? 121.4737}
          onConfirm={(lat, lng) => {
            onChange({ ...cfg, center_point: { lat, lng } } as any);
            setShowMap(false);
          }}
          onClose={() => setShowMap(false)}
        />
      )}
    </div>
  );
}
