package autoscaler

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const labelKey = "cloud"
const labelVal = "worker"

// docker scaler scales worker containers using the docker API
type DockerScaler struct {
	client *client.Client
	image  string
}

// new docker scaler creates a scaler that runs worker containers. image is the docker image name (e.g. cloud-worker)
func NewDockerScaler(image string) (*DockerScaler, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerScaler{client: cli, image: image}, nil
}

// start worker creates and starts a new worker container
func (d *DockerScaler) StartWorker(ctx context.Context) (string, error) {
	cfg := &container.Config{
		Image: d.image,
		Env: []string{
			"API_URL=http://host.docker.internal:8080",
			"WORKER_ENDPOINT=http://container:9090",
			"EXECUTION_BINARY=/app/runner",
		},
		Labels: map[string]string{labelKey: labelVal},
	}
	hostConfig := &container.HostConfig{
		AutoRemove: true,
	}
	resp, err := d.client.ContainerCreate(ctx, cfg, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}
	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// stop worker stops the container (auto remove will remove it)
func (d *DockerScaler) StopWorker(ctx context.Context, containerID string) error {
	var timeout int = 10
	return d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// worker container ids returns ids of containers with the worker label
func (d *DockerScaler) WorkerContainerIDs(ctx context.Context) ([]string, error) {
	list, err := d.client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", fmt.Sprintf("%s=%s", labelKey, labelVal))),
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, c := range list {
		ids = append(ids, c.ID)
	}
	return ids, nil
}
