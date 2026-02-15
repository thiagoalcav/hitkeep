import fs from "fs";
import path from "path";

const root = process.cwd();
const angularJsonPath = path.join(root, "angular.json");
const localeDir = path.join(root, "src", "locale");

if (!fs.existsSync(angularJsonPath)) {
    throw new Error("angular.json not found. Run this script from frontend/dashboard.");
}

const localeFiles = fs.existsSync(localeDir) ? fs.readdirSync(localeDir) : [];
const localeRegex = /^messages\.(.+)\.(xlf2?|xmb|json|arb)$/i;

const locales = {};
for (const file of localeFiles) {
    const match = file.match(localeRegex);
    if (!match) continue;
    const locale = match[1];
    locales[locale] = `src/locale/${file}`;
}

const angularJson = JSON.parse(fs.readFileSync(angularJsonPath, "utf8"));
const project = angularJson.projects?.dashboard;
if (!project) {
    throw new Error("dashboard project not found in angular.json");
}

project.i18n = project.i18n || {};
project.i18n.sourceLocale = project.i18n.sourceLocale || "en-US";
project.i18n.locales = locales;

fs.writeFileSync(angularJsonPath, JSON.stringify(angularJson, null, 4) + "\n");

const supported = Array.from(new Set([project.i18n.sourceLocale, ...Object.keys(locales)])).sort((a, b) => a.localeCompare(b, "en", { sensitivity: "base" }));

const supportedOut = [
    `export const SOURCE_LOCALE = '${project.i18n.sourceLocale}';`,
    `export const SUPPORTED_LOCALES = [`,
    ...supported.map((locale) => `    '${locale}',`),
    `] as const;`,
    ``,
    `export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];`,
    ``
].join("\n");

const supportedPath = path.join(root, "src", "app", "core", "i18n", "supported-locales.ts");
fs.writeFileSync(supportedPath, supportedOut);
