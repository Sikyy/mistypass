import i18next from "i18next"
import enUS from "./src/locales/en-US.json"

i18next.init({
  lng: "en-US",
  resources: { "en-US": { translation: enUS } },
  interpolation: { escapeValue: false },
})
