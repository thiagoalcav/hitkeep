export const SOURCE_LOCALE = 'en-US';
export const SUPPORTED_LOCALES = ['de', 'en-US'] as const;

export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];
