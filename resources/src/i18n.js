import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';

import Backend from 'i18next-http-backend';
//import LanguageDetector from 'i18next-browser-languagedetector';

i18n
    // load translation using http
    .use(Backend)
    // detect user language
    // .use(LanguageDetector)
    // pass the i18n instance to react-i18next.
    .use(initReactI18next)
    // init i18next
    .init({
        fallbackLng: 'fr',
        lng: 'fr',
        debug: false,
        backend: {
            loadPath: '/translations/{{lng}}.json',
        },
        interpolation: {
            escapeValue: false,
        },
        react: {
            wait: true,
            useSuspense: false,
        }
    });

export default i18n;
