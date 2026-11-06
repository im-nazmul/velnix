import js from "@eslint/js";
import tseslint from "typescript-eslint";
import vue from "eslint-plugin-vue";
import globals from "globals";
import prettier from "eslint-config-prettier";

export default [
    { ignores: ["node_modules/**", "dist/**", "playwright-report/**"] },
    js.configs.recommended,
    ...tseslint.configs.recommended,
    ...vue.configs["flat/recommended"],
    {
        files: ["**/*.{ts,vue}"],
        languageOptions: {
            parserOptions: { parser: tseslint.parser },
            globals: globals.browser,
        },
        rules: {
            "vue/multi-word-component-names": [
                "error",
                { ignores: ["App", "Icon"] },
            ],
            "@typescript-eslint/no-unused-vars": [
                "error",
                { argsIgnorePattern: "^_", caughtErrorsIgnorePattern: "^_" },
            ],
        },
    },
    {
        files: ["*.{mjs,ts}", "tests/**/*.ts"],
        languageOptions: { globals: globals.node },
    },
    // Formatting belongs to Prettier, not overlapping ESLint style rules.
    prettier,
];
