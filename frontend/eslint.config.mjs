import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

export default defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    // eslint-plugin-react-hooks' compiler-oriented rules flag patterns that are correct and used throughout (state set in an effect to
    // match the server's first paint, a clock read while rendering a relative time): warnings, to be cleaned up with the React compiler.
    rules: { "react-hooks/set-state-in-effect": "warn", "react-hooks/purity": "warn", "react-hooks/refs": "warn" },
  },
  globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts", "public/**", "scripts/**", "design-system/**"]),
]);
