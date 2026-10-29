import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { config } from "../../wailsjs/go/models";
import { GetLocalIPs, SelectFile } from "../../wailsjs/go/main/App";
import { detections, type DeviceId } from "../devices";
import { syncJointFaces } from "../jointDirection";
import {
  Advanced,
  Field,
  HostsField,
  NumberField,
  TextField,
  Toggle,
} from "./ConfigFields";
import Icon from "./Icon";
import JointDirectionEditor from "./JointDirectionEditor";

interface Props {
  cfg: config.Config;
  device: DeviceId;
  onChange: (cfg: config.Config) => void;
  onError: (error: unknown) => void;
  disabled: boolean;
  liveJointDirection: boolean;
}

export default function ConfigEditor({
  cfg,
  device,
  onChange,
  onError,
  disabled,
  liveJointDirection,
}: Props) {
  const { t } = useTranslation();
  const [localIPs, setLocalIPs] = useState<string[]>([]);
  const [detectionIndex, setDetectionIndex] = useState(0);
  const addressList = useId();
  const items = detections(cfg);
  const index = Math.min(detectionIndex, items.length - 1);
  const detection = items[index];

  useEffect(() => {
    let cancelled = false;
    GetLocalIPs()
      .then((ips) => {
        if (!cancelled) setLocalIPs(ips ?? []);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const patch = <K extends Exclude<DeviceId, "detection">>(
    id: K,
    value: Partial<config.Config[K]>,
  ) => {
    onChange(new config.Config({ ...cfg, [id]: { ...cfg[id], ...value } }));
  };
  const setDetections = (next: config.DetectionConfig[]) => {
    onChange(
      new config.Config({
        ...cfg,
        detections: next,
        detection: next[0],
        joint_direction: syncJointFaces(cfg.joint_direction, items, next),
      }),
    );
  };
  const patchDetection = (value: Partial<config.DetectionConfig>) => {
    setDetections(
      items.map((item, i) =>
        i === index ? new config.DetectionConfig({ ...item, ...value }) : item,
      ),
    );
  };

  const addDetection = () => {
    const last = items[items.length - 1];
    setDetections([
      ...items,
      new config.DetectionConfig({
        ...last,
        enabled: true,
        deviceID: last.deviceID + 1,
        port: last.port + 2,
      }),
    ]);
    setDetectionIndex(items.length);
  };

  const pickFile = async () => {
    try {
      const path = await SelectFile(
        cfg.analysis.o3_plus_o4_data_file ?? "",
        t("config.o3DataFile"),
      );
      if (path) patch("analysis", { o3_plus_o4_data_file: path });
    } catch (error) {
      onError(error);
    }
  };

  return (
    <div className="device-config">
      <datalist id={addressList}>
        {[...new Set(["0.0.0.0", "127.0.0.1", ...localIPs])].map((ip) => (
          <option key={ip} value={ip} />
        ))}
      </datalist>
      {device === "detection" && (
        <>
          <JointDirectionEditor
            settings={cfg.joint_direction}
            items={items}
            disabled={disabled}
            live={liveJointDirection}
            onChange={(joint_direction) =>
              onChange(new config.Config({ ...cfg, joint_direction }))
            }
          />
          <div className="device-toolbar">
            <select
              aria-label={t("config.detectionDevice")}
              value={index}
              onChange={(event) =>
                setDetectionIndex(Number(event.target.value))
              }
            >
              {items.map((item, i) => (
                <option key={i} value={i}>
                  {t("config.deviceNumber", { index: i + 1 })} · {item.deviceID}
                </option>
              ))}
            </select>
            <Toggle
              checked={detection.enabled}
              label={t("config.enableDevice")}
              onChange={(enabled) => patchDetection({ enabled })}
              disabled={disabled}
            />
            <button
              className="button button-quiet"
              onClick={addDetection}
              disabled={disabled}
            >
              <Icon name="plus" />
              {t("config.add")}
            </button>
            <button
              className="icon-button danger"
              aria-label={t("config.deleteDevice")}
              title={t("config.deleteDevice")}
              disabled={disabled || items.length <= 1}
              onClick={() => {
                setDetections(items.filter((_, i) => i !== index));
                setDetectionIndex(Math.max(0, index - 1));
              }}
            >
              <Icon name="trash" />
            </button>
          </div>
          <div className="form-grid">
            <TextField
              label={t("config.listenHost")}
              value={detection.host}
              onChange={(host) => patchDetection({ host })}
              list={addressList}
              disabled={disabled}
            />
            <NumberField
              label={t("config.port")}
              value={detection.port}
              onChange={(port) => patchDetection({ port })}
              min={1}
              max={65535}
              disabled={disabled}
            />
            <NumberField
              label={t("config.deviceID")}
              value={detection.deviceID}
              onChange={(deviceID) => patchDetection({ deviceID })}
              min={0}
              disabled={disabled}
            />
            <NumberField
              label={t("config.droneCount")}
              value={
                cfg.joint_direction.enabled
                  ? (cfg.joint_direction.targets?.length ?? 0)
                  : detection.drone_count
              }
              onChange={(drone_count) => patchDetection({ drone_count })}
              min={0}
              disabled={disabled || cfg.joint_direction.enabled}
            />
          </div>
          <Advanced>
            <NumberField
              label={t("config.heartbeatInterval")}
              value={detection.heartbeat_interval}
              onChange={(heartbeat_interval) =>
                patchDetection({ heartbeat_interval })
              }
              min={1}
              disabled={disabled}
            />
          </Advanced>
        </>
      )}

      {device === "analysis" && (
        <>
          <div className="form-grid">
            <HostsField
              value={cfg.analysis.hosts ?? []}
              onChange={(hosts) => patch("analysis", { hosts })}
              disabled={disabled}
            />
            <NumberField
              label={t("config.port")}
              value={cfg.analysis.port}
              onChange={(port) => patch("analysis", { port })}
              min={1}
              max={65535}
              disabled={disabled}
            />
            <TextField
              label={t("config.analysisDeviceIDs")}
              value={cfg.analysis.deviceID}
              placeholder="3000,3001"
              onChange={(deviceID) => patch("analysis", { deviceID })}
              disabled={disabled}
            />
            <NumberField
              label={t("config.droneCount")}
              value={cfg.analysis.drone_count}
              onChange={(drone_count) => patch("analysis", { drone_count })}
              min={0}
              disabled={disabled}
            />
          </div>
          <Advanced>
            <NumberField
              label={t("config.emptyPacketProb")}
              value={cfg.analysis.empty_packet_probability}
              onChange={(empty_packet_probability) =>
                patch("analysis", { empty_packet_probability })
              }
              min={0}
              max={100}
              disabled={disabled}
            />
            <Field label={t("config.o3DataFile")}>
              <div className="file-input">
                <input
                  aria-label={t("config.o3DataFile")}
                  value={cfg.analysis.o3_plus_o4_data_file ?? ""}
                  placeholder={t("config.randomData")}
                  readOnly
                  title={cfg.analysis.o3_plus_o4_data_file}
                />
                <button
                  className="button"
                  disabled={disabled}
                  onClick={() => void pickFile()}
                >
                  {t("config.choose")}
                </button>
                {cfg.analysis.o3_plus_o4_data_file && (
                  <button
                    className="icon-button"
                    aria-label={t("app.clear")}
                    disabled={disabled}
                    onClick={() =>
                      patch("analysis", { o3_plus_o4_data_file: undefined })
                    }
                  >
                    <Icon name="close" />
                  </button>
                )}
              </div>
            </Field>
          </Advanced>
        </>
      )}

      {device === "precision_df" && (
        <>
          <div className="form-grid">
            <div className="field-wide">
              <TextField
                label={t("config.listenHost")}
                value={cfg.precision_df.host}
                onChange={(host) => patch("precision_df", { host })}
                list={addressList}
                disabled={disabled}
              />
            </div>
            <NumberField
              label={t("config.commandPort")}
              value={cfg.precision_df.command_port}
              onChange={(command_port) =>
                patch("precision_df", { command_port })
              }
              min={1}
              max={65535}
              disabled={disabled}
            />
            <NumberField
              label={t("config.dataPort")}
              value={cfg.precision_df.data_port}
              onChange={(data_port) => patch("precision_df", { data_port })}
              min={1}
              max={65535}
              disabled={disabled}
            />
            <NumberField
              label={t("config.bearing")}
              value={cfg.precision_df.bearing_deg}
              onChange={(bearing_deg) => patch("precision_df", { bearing_deg })}
              min={-1}
              max={359.99}
              step={0.1}
              disabled={disabled}
            />
            <NumberField
              label={t("config.intervalMS")}
              value={cfg.precision_df.interval_ms}
              onChange={(interval_ms) => patch("precision_df", { interval_ms })}
              min={20}
              max={60000}
              disabled={disabled}
            />
          </div>
          <Advanced>
            <NumberField
              label={t("config.level")}
              value={cfg.precision_df.level_dbuv}
              onChange={(level_dbuv) => patch("precision_df", { level_dbuv })}
              min={-100}
              max={200}
              step={0.1}
              disabled={disabled}
            />
            <NumberField
              label={t("config.quality")}
              value={cfg.precision_df.quality}
              onChange={(quality) => patch("precision_df", { quality })}
              min={0}
              max={100}
              disabled={disabled}
            />
          </Advanced>
        </>
      )}

      {device === "directed_strike" && (
        <>
          <div className="form-grid">
            <TextField
              label={t("config.listenHost")}
              value={cfg.directed_strike.host}
              onChange={(host) => patch("directed_strike", { host })}
              list={addressList}
              disabled={disabled}
            />
            <NumberField
              label={t("config.port")}
              value={cfg.directed_strike.port}
              onChange={(port) => patch("directed_strike", { port })}
              min={1}
              max={65535}
              disabled={disabled}
            />
          </div>
          <Advanced>
            <NumberField
              label={t("config.responseDelay")}
              value={cfg.directed_strike.response_delay_ms}
              onChange={(response_delay_ms) =>
                patch("directed_strike", { response_delay_ms })
              }
              min={0}
              max={10000}
              disabled={disabled}
            />
          </Advanced>
        </>
      )}

      {(device === "fpv" || device === "jamming") && (
        <div className="form-grid">
          <HostsField
            value={cfg[device].hosts ?? []}
            onChange={(hosts) => patch(device, { hosts })}
            disabled={disabled}
          />
          <NumberField
            label={t("config.port")}
            value={cfg[device].port}
            onChange={(port) => patch(device, { port })}
            min={1}
            max={65535}
            disabled={disabled}
          />
          <NumberField
            label={t("config.deviceID")}
            value={cfg[device].deviceID}
            onChange={(deviceID) => patch(device, { deviceID })}
            min={0}
            disabled={disabled}
          />
        </div>
      )}
    </div>
  );
}
