import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Collapse, Select } from "antd";
import { config } from "../../wailsjs/go/models";
import { Advanced, Field, NumberField, TextField } from "./ConfigFields";
import Icon from "./Icon";

const MapPicker = lazy(() => import("./MapPicker"));

interface Props {
  cfg: config.Config;
  onChange: (cfg: config.Config) => void;
  disabled: boolean;
}

function TargetRow({
  drone,
  index,
  onChange,
  onDelete,
  disabled,
  initialOpen,
}: {
  drone: config.PredefinedDrone;
  index: number;
  onChange: (patch: Partial<config.PredefinedDrone>) => void;
  onDelete: () => void;
  disabled: boolean;
  initialOpen: boolean;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(initialOpen);
  const coordinates = (key: "drone_gps" | "pilot_gps") => (
    <>
      <NumberField
        label={t(key === "drone_gps" ? "config.droneLat" : "config.pilotLat")}
        value={drone[key]?.lat ?? 0}
        step="any"
        min={-90}
        max={90}
        disabled={disabled}
        onChange={(lat) => onChange({ [key]: { ...drone[key], lat } })}
      />
      <NumberField
        label={t(key === "drone_gps" ? "config.droneLng" : "config.pilotLng")}
        value={drone[key]?.lng ?? 0}
        step="any"
        min={-180}
        max={180}
        disabled={disabled}
        onChange={(lng) => onChange({ [key]: { ...drone[key], lng } })}
      />
    </>
  );

  return (
    <Collapse
      className="target-row"
      activeKey={open ? ["target"] : []}
      onChange={(keys) => setOpen(keys.includes("target"))}
      items={[{ key: "target", label: <div className="target-summary">
        <strong>
          {drone.model || t("config.targetNumber", { index: index + 1 })}
        </strong>
        <code>{drone.serial || "—"}</code>
      </div>, children: <div className="target-fields">
        <div className="form-grid">
          <TextField
            label={t("config.serial")}
            value={drone.serial}
            onChange={(serial) => onChange({ serial })}
            disabled={disabled}
          />
          <TextField
            label={t("config.model")}
            value={drone.model}
            onChange={(model) => onChange({ model })}
            disabled={disabled}
          />
          <NumberField
            label={t("config.freq")}
            value={drone.freq}
            onChange={(freq) => onChange({ freq })}
            min={0}
            step={0.01}
            disabled={disabled}
          />
          <NumberField
            label={t("config.rssi")}
            value={drone.rssi}
            onChange={(rssi) => onChange({ rssi })}
            disabled={disabled}
          />
          <Field label={t("config.type")}>
            <Select
              aria-label={t("config.type")}
              value={drone.type}
              onChange={(type) => onChange({ type })}
              disabled={disabled}
              options={[{ value: "AUTO", label: t("config.auto") }, { value: "DID", label: "DID" }, { value: "RID", label: "RID" }]}
            />
          </Field>
        </div>
        <div className="form-grid coordinate-grid">
          {coordinates("drone_gps")}
          {coordinates("pilot_gps")}
        </div>
        <Button type="text" danger
          className="button button-quiet danger"
          onClick={onDelete}
          disabled={disabled}
        >
          <Icon name="trash" />
          {t("config.deleteTarget")}
        </Button>
      </div> }]} />
  );
}

export default function TargetEditor({ cfg, onChange, disabled }: Props) {
  const { t } = useTranslation();
  const [showMap, setShowMap] = useState(false);
  const [newTarget, setNewTarget] = useState(-1);
  const patch = (value: Partial<config.Config>) =>
    onChange(new config.Config({ ...cfg, ...value }));
  const targets = cfg.predefined_drones ?? [];

  const add = () => {
    setNewTarget(targets.length);
    patch({
      predefined_drones: [
        ...targets,
        new config.PredefinedDrone({
          serial: "",
          model: "",
          freq: 2437,
          rssi: -70,
          type: "AUTO",
          drone_gps: cfg.center_point,
          pilot_gps: { lat: 0, lng: 0 },
        }),
      ],
    });
  };

  return (
    <>
      <div className="section-heading">
        <h2>{t("config.centerPoint")}</h2>
        <Button type="text"
          className="button button-quiet"
          disabled={disabled}
          onClick={() => setShowMap(true)}
        >
          <Icon name="map" />
          {t("config.map")}
        </Button>
      </div>
      <div className="form-grid">
        <NumberField
          label={t("config.lat")}
          value={cfg.center_point.lat}
          onChange={(lat) =>
            patch({ center_point: { ...cfg.center_point, lat } })
          }
          min={-90}
          max={90}
          step="any"
          disabled={disabled}
        />
        <NumberField
          label={t("config.lng")}
          value={cfg.center_point.lng}
          onChange={(lng) =>
            patch({ center_point: { ...cfg.center_point, lng } })
          }
          min={-180}
          max={180}
          step="any"
          disabled={disabled}
        />
        <NumberField
          label={t("config.minPushSpeed")}
          value={cfg.min_push_speed}
          onChange={(min_push_speed) => patch({ min_push_speed })}
          min={1}
          disabled={disabled}
        />
        <NumberField
          label={t("config.maxPushSpeed")}
          value={cfg.max_push_speed}
          onChange={(max_push_speed) => patch({ max_push_speed })}
          min={1}
          disabled={disabled}
        />
      </div>
      <Advanced>
        <NumberField
          label={t("config.refreshInterval")}
          value={cfg.random_drone_refresh_interval}
          onChange={(random_drone_refresh_interval) =>
            patch({ random_drone_refresh_interval })
          }
          min={0}
          disabled={disabled}
        />
        <NumberField
          label={t("config.maxDirectionChange")}
          value={cfg.max_direction_change}
          onChange={(max_direction_change) => patch({ max_direction_change })}
          min={0}
          max={360}
          step={0.1}
          disabled={disabled}
        />
        <NumberField
          label={t("config.maxDistance")}
          value={cfg.max_distance_from_center_point}
          onChange={(max_distance_from_center_point) =>
            patch({ max_distance_from_center_point })
          }
          min={0}
          step={0.1}
          disabled={disabled}
        />
      </Advanced>
      <section className="targets-section">
        <div className="section-heading">
          <h2>
            {t("config.targets")}
            <span className="count">{targets.length}</span>
          </h2>
          <Button type="text"
            className="button button-quiet"
            disabled={disabled}
            onClick={add}
          >
            <Icon name="plus" />
            {t("config.add")}
          </Button>
        </div>
        {targets.map((drone, index) => (
          <TargetRow
            key={index}
            drone={drone}
            index={index}
            disabled={disabled}
            initialOpen={index === newTarget}
            onChange={(value) =>
              patch({
                predefined_drones: targets.map((item, i) =>
                  i === index
                    ? new config.PredefinedDrone({ ...item, ...value })
                    : item,
                ),
              })
            }
            onDelete={() => {
              patch({
                predefined_drones: targets.filter((_, i) => i !== index),
              });
              setNewTarget(-1);
            }}
          />
        ))}
      </section>
      {showMap && (
        <Suspense fallback={null}>
          <MapPicker
            lat={cfg.center_point.lat}
            lng={cfg.center_point.lng}
            onClose={() => setShowMap(false)}
            onConfirm={(lat, lng) => {
              patch({ center_point: { lat, lng } });
              setShowMap(false);
            }}
          />
        </Suspense>
      )}
    </>
  );
}
