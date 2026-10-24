// @ts-check
//
// The client's lint, with types. typescript-eslint's type-checked rules are
// the ones Codacy runs, but Codacy runs them without installing the
// dependencies, so every Angular and d3 type is unresolved there and every use
// of one is flagged. Here they see the real types: npm ci, then ng lint —
// ./preflight.sh does it whenever node_modules is installed.
const eslint = require('@eslint/js');
const { defineConfig } = require('eslint/config');
const tseslint = require('typescript-eslint');
const angular = require('angular-eslint');

module.exports = defineConfig([
  {
    files: ['**/*.ts'],
    extends: [
      eslint.configs.recommended,
      tseslint.configs.strictTypeChecked,
      tseslint.configs.stylisticTypeChecked,
      angular.configs.tsRecommended,
    ],
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: __dirname },
    },
    processor: angular.processInlineTemplates,
    rules: {
      '@angular-eslint/directive-selector': ['error', { type: 'attribute', prefix: 'w', style: 'camelCase' }],
      '@angular-eslint/component-selector': ['error', { type: 'element', prefix: ['w', 'app'], style: 'kebab-case' }],
      '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
    },
  },
  {
    files: ['**/*.html'],
    extends: [angular.configs.templateRecommended, angular.configs.templateAccessibility],
    rules: {},
  },
]);
