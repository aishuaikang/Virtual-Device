import { config } from "../wailsjs/go/models";

export function faceRSSI(index: number, count: number, peak: number) {
  const distance = Math.min(
    Math.abs(index - peak),
    count - Math.abs(index - peak),
  );
  return -40 - Math.min(50, distance * 10);
}

export function adjustJointRSSI(
  faces: config.JointDirectionSample[],
  index: number,
  rssi: number,
  linked: boolean,
) {
  return faces.map((face, i) => {
    if (!linked && i !== index) return face;
    const offset = ((i - index) * Math.PI * 2) / faces.length;
    const attenuation = linked ? (45 * (1 - Math.cos(offset))) / 2 : 0;
    return new config.JointDirectionSample({
      ...face,
      rssi:
        Math.round(Math.min(0, Math.max(-140, rssi - attenuation)) * 10) / 10,
    });
  });
}

export function createJointTarget(
  items: config.DetectionConfig[],
  index: number,
) {
  const base = 5800 + index * 50;
  return new config.JointDirectionTarget({
    model: "O4",
    frequencies: [base, base + 3, base - 2, base + 6],
    faces: items.map((item, i) => ({
      deviceID: item.deviceID,
      reporting: true,
      rssi: faceRSSI(i, items.length, Math.min(2, items.length - 1)),
    })),
  });
}

// Keep per-face settings when modules are added, removed, or their hardware ID changes.
export function syncJointFaces(
  settings: config.JointDirectionConfig,
  previous: config.DetectionConfig[],
  next: config.DetectionConfig[],
) {
  return new config.JointDirectionConfig({
    ...settings,
    targets: (settings.targets ?? []).map((target) => ({
      ...target,
      faces: next.map((item, i) => {
        const oldID = previous[i]?.deviceID;
        const existing =
          previous.some((d) => d.deviceID === item.deviceID) ||
          (oldID !== undefined && !next.some((d) => d.deviceID === oldID));
        const source =
          (target.faces ?? []).find(
            (face) => face.deviceID === item.deviceID,
          ) ??
          (target.faces ?? []).find(
            (face) =>
              face.deviceID === oldID &&
              !next.some((d) => d.deviceID === oldID),
          );
        return {
          deviceID: item.deviceID,
          reporting: source?.reporting ?? !existing,
          rssi:
            source?.rssi ??
            faceRSSI(i, next.length, Math.min(2, next.length - 1)),
        };
      }),
    })),
  });
}
