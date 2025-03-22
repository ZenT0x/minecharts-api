package config

import (
	"os"
)

// Global configuration variables, configurable via environment variables.
var (
	DefaultNamespace = getEnv("MINECHARTS_NAMESPACE", "minecharts")
	DeploymentPrefix = getEnv("MINECHARTS_DEPLOYMENT_PREFIX", "minecraft-server-")
	PVCSuffix        = getEnv("MINECHARTS_PVC_SUFFIX", "-pvc")
	StorageSize      = getEnv("MINECHARTS_STORAGE_SIZE", "10Gi")
	StorageClass     = getEnv("MINECHARTS_STORAGE_CLASS", "rook-ceph-block")
	DefaultReplicas  = 1
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
