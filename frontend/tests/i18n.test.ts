import { describe, expect, test } from "bun:test";
import { createInstance } from "i18next";
import en from "../src/i18n/en";
import zh from "../src/i18n/zh";
import tr from "../src/i18n/tr";
import fr from "../src/i18n/fr";
import { errorReason } from "../src/i18n/errors";
import { localizeCommand } from "../src/i18n/runtime";

const messages = { zh, en, tr, fr };
function flatten(value: object, prefix = ""): Record<string, string> {
  return Object.fromEntries(Object.entries(value).flatMap(([key, entry]) => {
    const path = `${prefix}${key}`;
    return typeof entry === "string" ? [[path, entry]] : Object.entries(flatten(entry, `${path}.`));
  }));
}

function loadLanguage(configured: string, saved: string, browserLanguages = ["en-US"], next = "fr") {
  const modulePath = new URL("../src/i18n/index.ts", import.meta.url).href;
  const result = Bun.spawnSync({
    cmd: [process.execPath, "-e", `
      let saved = ${JSON.stringify(saved)};
      globalThis.window = {
        localStorage: { getItem: () => saved, setItem: (_, value) => { saved = value; } },
        navigator: { language: ${JSON.stringify(browserLanguages[0])}, languages: ${JSON.stringify(browserLanguages)} },
      };
      const m = await import(${JSON.stringify(modulePath)});
      const initial = m.getCurrentLanguage();
      await m.changeLanguage(${JSON.stringify(next)});
      console.log(JSON.stringify({ available: m.AVAILABLE_LANGUAGES, initial, current: m.getCurrentLanguage(), saved }));
    `],
    env: { ...process.env, VITE_APP_LANGUAGES: configured },
  });
  if (result.exitCode !== 0) throw new Error(result.stderr.toString());
  return JSON.parse(result.stdout.toString().trim().split("\n").at(-1)!);
}

describe("simulator internationalization", () => {
  const english = flatten(en);
  for (const [language, resource] of Object.entries(messages)) {
    test(`${language} covers every message and preserves its parameters`, () => {
      const entries = flatten(resource);
      expect(Object.keys(entries).sort()).toEqual(Object.keys(english).sort());
      for (const [key, value] of Object.entries(entries)) {
        expect(value.trim().length, key).toBeGreaterThan(0);
        expect(value.match(/{{.*?}}/g)?.sort() ?? [], key).toEqual(english[key].match(/{{.*?}}/g)?.sort() ?? []);
        if (language !== "zh") expect(value, key).not.toMatch(/\p{Script=Han}/u);
      }
    });
  }

  test("formats English and French connection counts", async () => {
    const instance = createInstance();
    await instance.init({ resources: { en: { translation: en }, fr: { translation: fr } }, lng: "en", fallbackLng: false });
    expect(instance.t("app.connections", { count: 1 })).toBe("1 connection");
    expect(instance.t("app.connections", { count: 2 })).toBe("2 connections");
    await instance.changeLanguage("fr");
    expect(instance.t("app.connections", { count: 0 })).toBe("0 connexion");
    expect(instance.t("app.connections", { count: 1 })).toBe("1 connexion");
    expect(instance.t("app.connections", { count: 2 })).toBe("2 connexions");
  });

  test("translates runtime command names and preserves protocol command data", () => {
    for (const resource of Object.values(messages)) {
      const entries = flatten(resource);
      expect(localizeCommand("心跳", (key) => entries[key])).toBe(resource.commands.heartbeat);
      expect(localizeCommand("云台俯仰角查询", (key) => entries[key])).toBe(resource.commands.pitchQuery);
      expect(localizeCommand("0x0F · IO1–IO15", (key) => entries[key])).toBe("0x0F · IO1–IO15");
      expect(localizeCommand("FREQ:MODE PSCAN2", (key) => entries[key])).toBe("FREQ:MODE PSCAN2");
      expect(localizeCommand("constructor", (key) => entries[key])).toBe("constructor");
    }
  });

  test("keeps the existing saved Chinese preference and persists new languages", () => {
    expect(loadLanguage("", "zh").initial).toBe("zh");
    for (const language of ["zh", "en", "tr", "fr"]) {
      const result = loadLanguage("", "", ["en-US"], language);
      expect(result.current).toBe(language);
      expect(result.saved).toBe(language);
      expect(loadLanguage("", result.saved).initial).toBe(language);
    }
  });

  test("resolves regional browser preferences and ignores unknown saved values", () => {
    expect(loadLanguage("", "unknown", ["fr-FR", "en-US"]).initial).toBe("fr");
    expect(loadLanguage("", "", ["tr-TR"]).initial).toBe("tr");
    expect(loadLanguage("", "", ["zh-CN"]).initial).toBe("zh");
    expect(loadLanguage("", "", ["de-DE"]).initial).toBe("en");
  });

  test("respects customer language restrictions and validates the build setting", () => {
    const french = loadLanguage("fr", "zh");
    expect(french.available).toEqual(["fr"]);
    expect(french.initial).toBe("fr");
    const restricted = loadLanguage("en,tr", "fr");
    expect(restricted.initial).toBe("en");
    expect(restricted.current).toBe("en");
    expect(loadLanguage("en,en,tr", "en").available).toEqual(["en", "tr"]);
    expect(() => loadLanguage("xx", "en")).toThrow("VITE_APP_LANGUAGES");
    expect(() => loadLanguage(",,", "en")).toThrow("VITE_APP_LANGUAGES");
  });
});

describe("localized error reasons", () => {
  for (const [message, key] of [
    ["listen tcp 0.0.0.0:9000: bind: address already in use", "errors.addressInUse"],
    ["open settings.json: permission denied", "errors.permission"],
    ["context deadline exceeded", "errors.timeout"],
    ["dial tcp: connection refused", "errors.connection"],
    ["运行中仅可修改联合测向各面的强度和上报开关，其他配置请先停止模拟器", "errors.runningConfig"],
    ["定向窄带模拟器配置错误: 监听端口必须在1到65535之间", "errors.invalidConfig"],
  ]) {
    test(message, () => {
      expect(errorReason(new Error(message))).toBe(key);
      for (const resource of Object.values(messages)) expect(flatten(resource)[key]).toBeTruthy();
    });
  }
  test("uses the operation message for unknown diagnostic details", () => {
    expect(errorReason("unexpected device response: 0x0102")).toBeUndefined();
  });
});
