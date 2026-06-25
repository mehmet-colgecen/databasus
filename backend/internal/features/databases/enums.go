package databases

type DatabaseType string

const (
	DatabaseTypePostgres   DatabaseType = "POSTGRES"
	DatabaseTypeRedis      DatabaseType = "REDIS"
	DatabaseTypeRabbitmq   DatabaseType = "RABBITMQ"
	DatabaseTypeKubernetes DatabaseType = "KUBERNETES"
)

type HealthStatus string

const (
	HealthStatusAvailable   HealthStatus = "AVAILABLE"
	HealthStatusUnavailable HealthStatus = "UNAVAILABLE"
)
