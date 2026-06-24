import eslint from "@eslint/js";
import { defineConfig } from "eslint/config";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";

export default defineConfig(
  { ignores: ["dist/**", "node_modules/**", "scripts/**"] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      parserOptions: {
        ecmaFeatures: { jsx: true },
      },
      globals: {
        ...globals.browser,
      },
    },
    plugins: {
      "react-hooks": reactHooks,
    },
    rules: {
      // Core hooks only — v7 recommended also enables React Compiler rules
      // (set-state-in-effect, refs, preserve-manual-memoization, etc.) that
      // this codebase has not been migrated to yet.
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    files: ["**/*.{test,spec}.{ts,tsx}"],
    languageOptions: {
      globals: {
        ...globals.browser,
        describe: "readonly",
        it: "readonly",
        expect: "readonly",
        vi: "readonly",
        beforeEach: "readonly",
        afterEach: "readonly",
        beforeAll: "readonly",
        afterAll: "readonly",
      },
    },
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
        },
      ],
      "max-lines": ["error", { max: 500, skipBlankLines: true, skipComments: true }],
      "no-restricted-syntax": [
        "error",
        {
          selector: "CallExpression[callee.name='setTimeout']",
          message: "Use vi.useFakeTimers() in tests — see docs/domain/web-testing.md",
        },
      ],
    },
  },
  {
    files: [
      "src/api/parseTaskApi.test.ts",
      "src/tasks/components/task-detail/cycles/TaskCyclesPanel.test.tsx",
      "src/tasks/components/task-list/section/TaskListSection.test.tsx",
    ],
    rules: {
      "max-lines": ["error", { max: 1100, skipBlankLines: true, skipComments: true }],
    },
  },
);
