import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zh from './zh';
import en from './en';

const LANG_KEY = 'virtual-device-ui.lang';

function resolveInitialLanguage() {
  if (typeof window === 'undefined') {
    return 'zh';
  }

  const saved = window.localStorage.getItem(LANG_KEY);
  if (saved === 'zh' || saved === 'en') {
    return saved;
  }

  const browserLang = window.navigator.language.toLowerCase();
  return browserLang.startsWith('zh') ? 'zh' : 'en';
}

i18n.use(initReactI18next).init({
  resources: {
    zh: { translation: zh },
    en: { translation: en },
  },
  lng: resolveInitialLanguage(),
  fallbackLng: 'en',
  supportedLngs: ['zh', 'en'],
  interpolation: { escapeValue: false },
});

if (typeof window !== 'undefined') {
  i18n.on('languageChanged', language => {
    if (language === 'zh' || language === 'en') {
      window.localStorage.setItem(LANG_KEY, language);
    }
  });
}

export default i18n;
