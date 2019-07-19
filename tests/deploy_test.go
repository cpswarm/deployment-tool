package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	endpoint     = "http://localhost:8080"
	elasticPort  = "9090"
	imageElastic = "elasticsearch:6.6.1"
	imageManager = "linksmart/deployment-manager"
	imageAgent   = "linksmart/deployment-agent"
)

// TODO create user defined network

func TestDeploy(t *testing.T) {

	t.Run("run elastic", func(t *testing.T) {
		tearDown, err := runElastic(t)
		defer tearDown()
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("run manager", func(t *testing.T) {
		tearDown, err := runManager(t)
		defer tearDown()
		if err != nil {
			t.Fatal(err)
		}
	})

	var err error
	var token string
	t.Run("get a token", func(t *testing.T) {
		token, err = getToken()
		if err != nil {
			t.Fatalf("%s", err)
		}
	})
	t.Log(token)

	t.Run("run an agent", func(t *testing.T) {
		tearDown, err := runAgent(t, token)
		defer tearDown()
		if err != nil {
			t.Fatal(err)
		}
	})

}

func getToken() (string, error) {
	resp, err := http.Post(endpoint+"/token_sets?name=test&total=1", "none", nil)
	if err != nil {
		return "", fmt.Errorf("error requesting token: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201 but got %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	respMap := make(map[string]interface{})
	err = decoder.Decode(&respMap)
	if err != nil {
		return "", fmt.Errorf("error decoding response: %s", err)
	}

	if _, found := respMap["tokens"]; !found {
		return "", fmt.Errorf("tokens not found in response:\n%s", spew.Sdump(respMap))
	}

	tokens, ok := respMap["tokens"].([]interface{})
	if !ok {
		return "", fmt.Errorf("type assertion not possible for tokens in response:\n%s", spew.Sdump(respMap))
	}

	if len(tokens) != 1 {
		return "", fmt.Errorf("expected 1 token but got %d", len(tokens))
	}

	token, ok := tokens[0].(string)
	if !ok {
		return "", fmt.Errorf("type assertion not possible for token in response:\n%s", spew.Sdump(respMap))
	}

	return token, nil
}

func runElastic(t *testing.T) (func(), error) {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	_, err = cli.ImagePull(ctx, imageElastic, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println("Pulled image:", imageElastic)

	// container to generate key pair
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageElastic,
			Env: []string{
				"discovery.type=single-node",
				"bootstrap.memory_lock=true",
				"ES_JAVA_OPTS=-Xms512m -Xmx512m",
			},
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				"9200/tcp": []nat.PortBinding{
					{
						HostIP:   "127.0.0.1",
						HostPort: elasticPort,
					},
				},
			},
		},
		nil,
		"")
	if err != nil {
		panic(err)
	}
	fmt.Println("Created container:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
	fmt.Println("Started container:", resp.ID)

	return func() {
		containerStop(t, cli, ctx, resp.ID)
	}, nil
}

func runManager(t *testing.T) (func(), error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cli.ImagePull(ctx, imageManager, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("Pulled image:", imageManager)

	workDir, _ := os.Getwd()
	mountPoint := workDir + "/volumes/manager"
	err = os.MkdirAll(mountPoint, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	mountAgent := mount.Mount{
		Type:   mount.TypeBind,
		Source: mountPoint,
		Target: "/home/keys",
	}

	// container to generate key pair
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageManager,
			Cmd:   []string{"-newkeypair", "keys/manager"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{mountAgent},
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Created container for key pair generation:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Started container:", resp.ID)

	fmt.Println("Waiting for container to exit...")
	waitOK, _ := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	if body := <-waitOK; body.Error != nil {
		t.Fatal(body.Error)
	}
	fmt.Println("Container exited:", resp.ID)
	containerLogs(t, cli, ctx, resp.ID)

	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Removed container:", resp.ID)

	// actual runtime container
	resp, err = cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageManager,
			Env:   []string{"STORAGE_DSN=http://localhost:" + elasticPort},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{mountAgent},
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Created container:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Started container:", resp.ID)
	return func() {
		containerStop(t, cli, ctx, resp.ID)
	}, nil
}

func runAgent(t *testing.T, token string) (func(), error) {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cli.ImagePull(ctx, imageAgent, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Pulled image:", imageAgent)

	workDir, _ := os.Getwd()
	mountPoint := workDir + "/volumes/agent"
	err = os.MkdirAll(mountPoint, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	mountAgent := mount.Mount{
		Type:   mount.TypeBind,
		Source: mountPoint,
		Target: "/home/agent",
	}

	// container to generate key pair
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageAgent,
			Cmd:   []string{"-newkeypair", "agent"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{mountAgent},
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Created container for key pair generation:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Started container:", resp.ID)

	fmt.Println("Waiting for container to exit...")
	waitOK, _ := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	if body := <-waitOK; body.Error != nil {
		t.Fatal(body.Error)
	}
	fmt.Println("Container exited:", resp.ID)

	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Removed container:", resp.ID)

	// actual runtime container
	resp, err = cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageAgent,
			Env:   []string{"AUTH_TOKEN=" + token, "MANAGER_ADDR=" + endpoint},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{mountAgent},
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Created container:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Started container:", resp.ID)

	return func() {
		containerStop(t, cli, ctx, resp.ID)
	}, nil
}

func containerStop(t *testing.T, cli *client.Client, ctx context.Context, id string) {
	if err := cli.ContainerStop(ctx, id, nil); err != nil {
		t.Fatalf("%s", err)
	}
	fmt.Println("Stopped container:", id)

	if t.Failed() {
		containerLogs(t, cli, ctx, id)
	}

	if err := cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Removed container:", id)
}

func containerLogs(t *testing.T, cli *client.Client, ctx context.Context, id string) {

	reader, err := cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	logs, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Printing container logs for: %s\n%s", id, logs)
}
