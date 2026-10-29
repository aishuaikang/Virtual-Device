import { useRef, useState, type PointerEvent } from "react";
import { useTranslation } from "react-i18next";
import { Checkbox } from "antd";
import { config } from "../../wailsjs/go/models";

const center = 160;
const radius = 106;
const minimum = -140;

function point(index: number, count: number, distance: number) {
  const angle = (index * Math.PI * 2) / count;
  return {
    x: center + Math.sin(angle) * distance,
    y: center - Math.cos(angle) * distance,
  };
}

export default function JointDirectionRadar({
  items,
  faces,
  disabled,
  onChange,
}: {
  items: config.DetectionConfig[];
  faces: config.JointDirectionSample[];
  disabled: boolean;
  onChange: (index: number, rssi: number, linked: boolean) => void;
}) {
  const { t } = useTranslation();
  const [linked, setLinked] = useState(true);
  const [active, setActive] = useState<number | null>(null);
  const drag = useRef<{ index: number; pointerID: number } | null>(null);
  const available = (i: number) => items[i].enabled && faces[i].reporting;
  const distance = (i: number) =>
    available(i)
      ? Math.max(0, Math.min(1, (faces[i].rssi - minimum) / -minimum)) * radius
      : 0;
  const vertices = (distanceFor: (i: number) => number) =>
    items
      .map((_, i) => {
        const p = point(i, items.length, distanceFor(i));
        return `${p.x},${p.y}`;
      })
      .join(" ");
  const move = (event: PointerEvent<SVGSVGElement>) => {
    const current = drag.current;
    if (!current || current.pointerID !== event.pointerId || disabled) return;
    const matrix = event.currentTarget.getScreenCTM();
    if (!matrix) return;
    const p = event.currentTarget.createSVGPoint();
    p.x = event.clientX;
    p.y = event.clientY;
    const local = p.matrixTransform(matrix.inverse());
    const angle = (current.index * Math.PI * 2) / items.length;
    const projection =
      (local.x - center) * Math.sin(angle) -
      (local.y - center) * Math.cos(angle);
    const rssi =
      minimum + Math.max(0, Math.min(1, projection / radius)) * -minimum;
    onChange(current.index, rssi, linked);
  };
  const endDrag = (event: PointerEvent<SVGSVGElement>) => {
    if (drag.current?.pointerID !== event.pointerId) return;
    drag.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  };

  return (
    <div className="joint-radar">
      <div className="joint-radar-heading">
        <strong>{t("joint.signalRadar")}</strong>
        <Checkbox className="joint-link-toggle"
            checked={linked}
            onChange={(event) => setLinked(event.target.checked)}
          >
          {t("joint.linkFaces")}
        </Checkbox>
      </div>
      <svg
        className="joint-radar-chart"
        viewBox="0 0 320 320"
        aria-label={t("joint.signalRadar")}
        onPointerMove={move}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onLostPointerCapture={() => {
          drag.current = null;
        }}
      >
        <title>{t("joint.radarHint")}</title>
        {[0.25, 0.5, 0.75, 1].map((scale) => (
          <g key={scale} className="joint-radar-grid">
            {items.length >= 3 ? (
              <polygon points={vertices(() => radius * scale)} />
            ) : (
              <circle cx={center} cy={center} r={radius * scale} />
            )}
            <text x={center + 5} y={center - radius * scale - 3}>
              {minimum + scale * -minimum}
            </text>
          </g>
        ))}
        {items.map((item, i) => {
          const end = point(i, items.length, radius);
          const label = point(i, items.length, radius + 29);
          const showLabel =
            items.length <= 12 ||
            i % Math.ceil(items.length / 12) === 0 ||
            active === i;
          return (
            <g
              key={item.deviceID}
              className={`joint-radar-axis${available(i) ? "" : " is-missing"}`}
            >
              <line x1={center} y1={center} x2={end.x} y2={end.y} />
              {showLabel && (
                <text x={label.x} y={label.y - 2} textAnchor="middle">
                  {item.deviceID}
                  <tspan x={label.x} dy="13">
                    {Number(((i * 360) / items.length).toFixed(1))}°
                  </tspan>
                </text>
              )}
            </g>
          );
        })}
        <polygon className="joint-radar-signal" points={vertices(distance)} />
        {items.length < 3 &&
          items.map((item, i) => {
            const p = point(i, items.length, distance(i));
            return (
              <line
                key={item.deviceID}
                className="joint-radar-signal-line"
                x1={center}
                y1={center}
                x2={p.x}
                y2={p.y}
              />
            );
          })}
        {items.map((item, i) => {
          if (!available(i)) return null;
          const p = point(i, items.length, distance(i));
          return (
            <circle
              key={item.deviceID}
              className={`joint-radar-handle${active === i ? " is-active" : ""}`}
              cx={p.x}
              cy={p.y}
              r={7}
              role="slider"
              tabIndex={disabled ? -1 : 0}
              aria-label={t("joint.rssiFor", { id: item.deviceID })}
              aria-valuemin={minimum}
              aria-valuemax={0}
              aria-valuenow={faces[i].rssi}
              aria-valuetext={`${faces[i].rssi} dBm`}
              aria-disabled={disabled}
              onFocus={() => setActive(i)}
              onPointerDown={(event) => {
                if (disabled || event.button !== 0) return;
                event.preventDefault();
                event.currentTarget.focus({ preventScroll: true });
                drag.current = { index: i, pointerID: event.pointerId };
                setActive(i);
                event.currentTarget.ownerSVGElement?.setPointerCapture(
                  event.pointerId,
                );
              }}
              onKeyDown={(event) => {
                if (disabled) return;
                const step = event.shiftKey ? 5 : 1;
                const value =
                  event.key === "Home"
                    ? minimum
                    : event.key === "End"
                      ? 0
                      : ["ArrowUp", "ArrowRight"].includes(event.key)
                        ? faces[i].rssi + step
                        : ["ArrowDown", "ArrowLeft"].includes(event.key)
                          ? faces[i].rssi - step
                          : null;
                if (value === null) return;
                event.preventDefault();
                onChange(i, Math.max(minimum, Math.min(0, value)), linked);
              }}
            >
              <title>
                {item.deviceID} · {faces[i].rssi} dBm
              </title>
            </circle>
          );
        })}
        <circle className="joint-radar-center" cx={center} cy={center} r={2} />
      </svg>
      <p className="joint-radar-readout">
        {active !== null && faces[active] ? (
          <>
            <code>{faces[active].deviceID}</code>
            <strong>
              {faces[active].rssi.toFixed(1)} <small>dBm</small>
            </strong>
          </>
        ) : (
          <span>{t("joint.dragNode")}</span>
        )}
      </p>
      <p className="joint-hint">
        {t(linked ? "joint.linkedHint" : "joint.independentHint")}
      </p>
    </div>
  );
}
