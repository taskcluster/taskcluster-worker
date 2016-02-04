package config

type Config struct {
	Taskcluster struct {
		ClientId    string
		AccessToken string
		Certificate string
	}
	ProvisionerId string
	WorkerGroup   string
	WorkerId      string
	Capacity      int
	QueueService  struct {
		ExpirationOffset int
	}
}
