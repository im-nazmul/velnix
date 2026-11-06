export default {
    extends: ["stylelint-config-standard"],
    // Disjoint component selectors intentionally share element types. Preserve
    // their cascade order instead of rearranging unrelated components.
    rules: { "no-descending-specificity": null },
    ignoreFiles: ["node_modules/**", "../internal/dashboardassets/dist/**"],
    overrides: [{ files: ["**/*.vue"], customSyntax: "postcss-html" }],
};
