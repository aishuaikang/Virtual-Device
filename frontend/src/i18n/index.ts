import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zh from './zh';
import en from './en';
import tr from './tr';
import fr from './fr';

const LANG_KEY = 'virtual-device-ui.lang';
export const SUPPORTED_LANGUAGES = ['zh', 'en', 'tr', 'fr'] as const;
export type SupportedLanguage = typeof SUPPORTED_LANGUAGES[number];
export const LANGUAGE_LABELS: Record<SupportedLanguage, string> = {
  zh: '中文', en: 'English', tr: 'Türkçe', fr: 'Français',
};

export function getAvailableLanguages(configured?: string): readonly SupportedLanguage[] {
  if (!configured?.trim()) return SUPPORTED_LANGUAGES;
  const languages = configured.split(',').map(language => language.trim()).filter(Boolean);
  if (!languages.length || languages.some(language => !SUPPORTED_LANGUAGES.includes(language as SupportedLanguage))) {
    throw new Error('VITE_APP_LANGUAGES must contain supported language codes: zh,en,tr,fr');
  }
  return [...new Set(languages)] as SupportedLanguage[];
}

export const AVAILABLE_LANGUAGES = getAvailableLanguages(import.meta.env.VITE_APP_LANGUAGES);
const isAvailableLanguage = (language: string): language is SupportedLanguage =>
  AVAILABLE_LANGUAGES.includes(language as SupportedLanguage);
const DEFAULT_LANGUAGE = AVAILABLE_LANGUAGES.includes('en') ? 'en' : AVAILABLE_LANGUAGES[0];

function resolveInitialLanguage() {
  if (typeof window === 'undefined') {
    return DEFAULT_LANGUAGE;
  }

  const saved = window.localStorage.getItem(LANG_KEY);
  if (saved && isAvailableLanguage(saved)) {
    return saved;
  }

  for (const language of window.navigator.languages ?? [window.navigator.language]) {
    const base = language.toLowerCase().split(/[-_]/)[0];
    if (isAvailableLanguage(base)) return base;
  }
  return DEFAULT_LANGUAGE;
}

i18n.use(initReactI18next).init({
  resources: {
    zh: { translation: zh },
    en: { translation: en },
    tr: { translation: tr },
    fr: { translation: fr },
  },
  lng: resolveInitialLanguage(),
  fallbackLng: DEFAULT_LANGUAGE,
  supportedLngs: AVAILABLE_LANGUAGES,
  interpolation: { escapeValue: false },
});

if (typeof window !== 'undefined') {
  i18n.on('languageChanged', language => {
    if (isAvailableLanguage(language)) {
      window.localStorage.setItem(LANG_KEY, language);
    }
  });
}

export const getCurrentLanguage = (): SupportedLanguage =>
  isAvailableLanguage(i18n.language) ? i18n.language : DEFAULT_LANGUAGE;

export const changeLanguage = (language: SupportedLanguage) => {
  if (isAvailableLanguage(language)) return i18n.changeLanguage(language);
};

export default i18n;
