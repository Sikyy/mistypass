import eslint from "@eslint/js"
import tseslint from "typescript-eslint"
import reactHooks from "eslint-plugin-react-hooks"
import jsxA11y from "eslint-plugin-jsx-a11y"
import eslintConfigPrettier from "eslint-config-prettier"

export default tseslint.config(
  {
    ignores: ["node_modules/", "dist/"],
  },
  eslint.configs.recommended,
  ...tseslint.configs.strict,
  reactHooks.configs.flat.recommended,
  jsxA11y.flatConfigs.recommended,
  eslintConfigPrettier
)
