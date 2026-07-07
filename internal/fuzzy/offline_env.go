package fuzzy

// OfflineEnvKey is set in isolate sessions when --offline is active.
// Catalog setup tasks may branch on it (e.g. uv/pnpm/mvn offline flags).
const OfflineEnvKey = "RFT_FUZZY_OFFLINE"

// OfflineSessionEnv returns env vars for package managers and mise when offline.
func OfflineSessionEnv() map[string]string {
	return map[string]string{
		OfflineEnvKey:         "1",
		"MISE_OFFLINE":         "1",
		"GOPROXY":              "off",
		"GOSUMDB":              "off",
		"UV_OFFLINE":           "1",
		"NPM_CONFIG_OFFLINE":   "true",
		"PNPM_OFFLINE":         "true",
		"npm_config_prefer_offline": "true",
	}
}
