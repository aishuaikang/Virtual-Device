import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Select } from "antd";
import { config } from "../../wailsjs/go/models";
import {
  adjustJointRSSI,
  createJointTarget,
  faceRSSI,
} from "../jointDirection";
import { ClearableNumberInput, ClearableTextInput } from "./ClearableInput";
import { Field, NumberField, TextField, Toggle } from "./ConfigFields";
import Icon from "./Icon";
import JointDirectionRadar from "./JointDirectionRadar";

function FrequencyList({
  value,
  onChange,
  disabled,
}: {
  value: number[];
  onChange: (value: number[]) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  const text = value.join(", ");
  const [draft, setDraft] = useState(text);
  const focused = useRef(false);
  useEffect(() => {
    if (!focused.current) setDraft(text);
  }, [text]);
  return (
    <Field label={t("joint.frequencies")}>
      <ClearableTextInput
        value={draft}
        aria-label={t("joint.frequencies")}
        placeholder="5800, 5803, 5798, 5806"
        disabled={disabled}
        onFocus={() => {
          focused.current = true;
        }}
        onBlur={() => {
          focused.current = false;
          setDraft(text);
        }}
        onValueChange={(next) => {
          setDraft(next);
          onChange(
            next
              .split(/[,，\s]+/)
              .filter(Boolean)
              .map((part) => {
                const number = Number(part);
                return Number.isFinite(number) ? number : 0;
              }),
          );
        }}
      />
    </Field>
  );
}

type Preset = "all" | "missing" | "single" | "close" | "none";

export default function JointDirectionEditor({
  settings,
  items,
  onChange,
  disabled,
  live,
}: {
  settings: config.JointDirectionConfig;
  items: config.DetectionConfig[];
  onChange: (settings: config.JointDirectionConfig) => void;
  disabled: boolean;
  live: boolean;
}) {
  const { t } = useTranslation();
  const samplesDisabled = disabled && !live;
  const [selected, setSelected] = useState(0);
  const targets = settings.targets ?? [];
  const index = Math.min(selected, Math.max(0, targets.length - 1));
  const target = targets[index];
  const faces = items.map(
    (item) =>
      new config.JointDirectionSample(
        target?.faces?.find((face) => face.deviceID === item.deviceID) ?? {
          deviceID: item.deviceID,
          reporting: false,
          rssi: -70,
        },
      ),
  );
  const patch = (value: Partial<config.JointDirectionConfig>) =>
    onChange(new config.JointDirectionConfig({ ...settings, ...value }));
  const patchTarget = (value: Partial<config.JointDirectionTarget>) =>
    patch({
      targets: targets.map((item, i) =>
        i === index
          ? new config.JointDirectionTarget({ ...item, ...value })
          : item,
      ),
    });
  const ranked = faces
    .map((face, i) => ({ ...face, index: i }))
    .filter((face) => items[face.index].enabled)
    .sort((a, b) => b.rssi - a.rssi);
  const strongest = ranked.find((face) => face.reporting);
  const angle = (i: number) => Number(((i * 360) / items.length).toFixed(1));
  const preset = (mode: Preset) => {
    const peak = ranked[0]?.index ?? 0;
    const missing = new Set(ranked.slice(-2).map((face) => face.deviceID));
    patchTarget({
      faces: faces.map(
        (face, i) =>
          new config.JointDirectionSample({
            ...face,
            reporting:
              mode === "none"
                ? false
                : mode === "single"
                  ? i === peak
                  : mode === "missing"
                    ? !missing.has(face.deviceID)
                    : true,
            rssi:
              mode === "close"
                ? -40 - Math.abs(i - peak) * 0.2
                : faceRSSI(i, items.length, peak),
          }),
      ),
    });
  };

  return (
    <section className="joint-direction">
      <div className="section-heading">
        <h2>{t("joint.title")}</h2>
        <Toggle
          checked={settings.enabled}
          label={t("joint.title")}
          disabled={disabled}
          onChange={(enabled) =>
            patch({
              enabled,
              targets:
                enabled && !targets.length
                  ? [createJointTarget(items, 0)]
                  : targets,
            })
          }
        />
      </div>
      {settings.enabled && (
        <>
          <p className="joint-hint">{t("joint.setupHint")}</p>
          <div className="joint-target-toolbar">
            <Select<number>
              aria-label={t("joint.target")}
              value={index}
              onChange={setSelected}
              options={targets.map((item, i) => ({ value: i,
                label: `${t("config.targetNumber", { index: i + 1 })} · ${item.model}`,
              }))}
            />
            <Button type="text"
              className="button button-quiet"
              disabled={disabled}
              onClick={() => {
                let band = targets.length;
                while (
                  targets.some((item) =>
                    (item.frequencies ?? []).some(
                      (freq) => Math.abs(freq - (5800 + band * 50)) < 20,
                    ),
                  )
                )
                  band++;
                patch({
                  targets: [...targets, createJointTarget(items, band)],
                });
                setSelected(targets.length);
              }}
            >
              <Icon name="plus" />
              {t("config.add")}
            </Button>
            <Button type="text" danger
              className="icon-button danger"
              aria-label={t("config.deleteTarget")}
              title={t("config.deleteTarget")}
              disabled={disabled || targets.length <= 1}
              onClick={() => {
                patch({ targets: targets.filter((_, i) => i !== index) });
                setSelected(Math.max(0, index - 1));
              }}
            >
              <Icon name="trash" />
            </Button>
          </div>
          {target && (
            <>
              <div className="form-grid">
                <TextField
                  label={t("config.model")}
                  value={target.model}
                  onChange={(model) => patchTarget({ model })}
                  disabled={disabled}
                />
                <NumberField
                  label={t("joint.hopInterval")}
                  value={settings.hop_interval_ms}
                  onChange={(hop_interval_ms) => patch({ hop_interval_ms })}
                  min={100}
                  max={60000}
                  disabled={disabled}
                />
                <div className="field-wide">
                  <FrequencyList
                    key={index}
                    value={target.frequencies ?? []}
                    onChange={(frequencies) => patchTarget({ frequencies })}
                    disabled={disabled}
                  />
                </div>
              </div>
              <div className="joint-samples">
                <JointDirectionRadar
                  items={items}
                  faces={faces}
                  disabled={samplesDisabled}
                  onChange={(i, rssi, linked) =>
                    patchTarget({
                      faces: adjustJointRSSI(faces, i, rssi, linked),
                    })
                  }
                />
                <div className="joint-sample-controls">
                  {live && (
                    <p className="joint-live" role="status">
                      {t("joint.liveHint")}
                    </p>
                  )}
                  <div
                    className="joint-presets"
                    aria-label={t("joint.presets")}
                  >
                    {(
                      ["all", "missing", "single", "close", "none"] as const
                    ).map((mode) => (
                      <Button type="default"
                        key={mode}
                        className="button"
                        onClick={() => preset(mode)}
                        disabled={
                          samplesDisabled ||
                          (mode === "missing" && ranked.length < 3)
                        }
                      >
                        {t(`joint.${mode}`)}
                      </Button>
                    ))}
                  </div>
                  <div className="joint-table-wrap">
                    <table className="joint-table">
                      <thead>
                        <tr>
                          <th>{t("joint.hardwareID")}</th>
                          <th>UDP</th>
                          <th>{t("joint.relativeAngle")}</th>
                          <th>RSSI (dBm)</th>
                          <th>{t("joint.reporting")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {items.map((item, i) => (
                          <tr
                            key={i}
                            className={item.enabled ? "" : "is-disabled"}
                          >
                            <td>
                              <code>{item.deviceID}</code>
                            </td>
                            <td>
                              <code>{item.port}</code>
                            </td>
                            <td>{angle(i)}°</td>
                            <td>
                              <ClearableNumberInput
                                value={faces[i].rssi}
                                aria-label={t("joint.rssiFor", {
                                  id: item.deviceID,
                                })}
                                min={-140}
                                max={0}
                                step={0.1}
                                disabled={samplesDisabled}
                                onValueChange={(rssi) =>
                                  patchTarget({
                                    faces: adjustJointRSSI(
                                      faces,
                                      i,
                                      rssi,
                                      false,
                                    ),
                                  })
                                }
                              />
                            </td>
                            <td>
                              <Toggle
                                checked={faces[i].reporting}
                                disabled={samplesDisabled}
                                label={t("joint.reportingFor", {
                                  id: item.deviceID,
                                })}
                                onChange={(reporting) =>
                                  patchTarget({
                                    faces: faces.map((face, j) =>
                                      i === j
                                        ? new config.JointDirectionSample({
                                            ...face,
                                            reporting,
                                          })
                                        : face,
                                    ),
                                  })
                                }
                              />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <p className="joint-hint joint-result">
                    {strongest
                      ? t("joint.strongest", {
                          id: strongest.deviceID,
                          angle: angle(strongest.index),
                        })
                      : t("joint.noSamples")}
                  </p>
                </div>
              </div>
            </>
          )}
        </>
      )}
    </section>
  );
}
