import i18n from "i18next"
import LanguageDetector from "i18next-browser-languagedetector"
import { initReactI18next } from "react-i18next"

import enUS from "@/locales/en-US.json"
import idID from "@/locales/id-ID.json"
import zhCN from "@/locales/zh-CN.json"

const zh = { translation: zhCN }
const en = { translation: enUS }
const id = { translation: idID }

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      zh,
      "zh-CN": zh,
      en,
      "en-US": en,
      id,
      "id-ID": id,
    },
    fallbackLng: "zh-CN",
    supportedLngs: ["zh", "zh-CN", "en", "en-US", "id", "id-ID"],
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["querystring", "localStorage", "navigator"],
      lookupQuerystring: "lang",
      caches: ["localStorage"],
    },
  })

export default i18n
