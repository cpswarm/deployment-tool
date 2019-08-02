package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	userDefinedNetwork = "test-network"
	// elastic
	elasticImage = "elasticsearch:6.6.1"
	elasticName  = "test-elastic"
	elasticPort  = "9200"
	// manager
	managerImage = "linksmart/deployment-manager"
	managerName  = "test-manager"
	managerPort  = "8080"
	// agent
	agentImage = "linksmart/deployment-agent"
	agentName  = "test-agent"
	// test files
	deployOrder = "https://raw.githubusercontent.com/cpswarm/deployment-tool/master/examples/orders/zip-deploy.yml"
)

var (
	elasticEndpoint        = "http://" + elasticName + ":" + elasticPort
	managerEndpoint        = "http://" + managerName + ":" + managerPort
	managerExposedEndpoint = "http://localhost:" + managerPort
	testDir                string
)

func TestDeploy(t *testing.T) {

	var tearDownFuncs []func(*testing.T)

	// prepare the work directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("Error getting work directory:", err)
	}
	testDir = fmt.Sprintf("%s/volumes/%d", wd, time.Now().Unix())
	err = os.MkdirAll(testDir, os.ModePerm)
	if err != nil {
		t.Fatal("Error creating test dir:", err)
	}

	// create docker client
	ctx := context.Background()
	var ops []func(*client.Client) error
	if version := os.Getenv("DOCKER_API"); version != "" {
		ops = append(ops, client.WithVersion(version))
	}
	cli, err := client.NewClientWithOpts(ops...)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("create network", func(t *testing.T) {
		tearDown := createNetwork(t, cli, ctx)
		tearDownFuncs = append(tearDownFuncs, tearDown)
	})

	t.Run("run elastic", func(t *testing.T) {
		tearDown := runElastic(t, cli, ctx)
		tearDownFuncs = append(tearDownFuncs, tearDown)
	})

	t.Run("run manager", func(t *testing.T) {
		tearDown := runManager(t, cli, ctx)
		tearDownFuncs = append(tearDownFuncs, tearDown)
	})

	var token string
	t.Run("get token", func(t *testing.T) {
		token = getToken(t)
	})
	t.Log(token)

	t.Run("run agent", func(t *testing.T) {
		tearDown := runAgent(t, cli, ctx, token)
		tearDownFuncs = append(tearDownFuncs, tearDown)
	})

	time.Sleep(5 * time.Second) // wait for the registration by agent

	t.Run("check registration", func(t *testing.T) {
		checkRegistration(t)
	})

	t.Run("deploy package", func(t *testing.T) {
		deployPackage(t)
		time.Sleep(30 * time.Second)
	})

	t.Run("check log reports", func(t *testing.T) {
		t.SkipNow()
	})

	t.Run("check deployed files", func(t *testing.T) {
		t.SkipNow()
	})

	t.Log("Starting to tear down.")
	t.Run("remove volumes", func(t *testing.T) {
		removeVolumes(t, cli, ctx)
	})

	for i := len(tearDownFuncs) - 1; i >= 0; i-- {
		tearDownFuncs[i](t)
	}

	// delete data
	err = os.RemoveAll(testDir)
	if err != nil {
		t.Fatal("Error removing test files:", err)
	}
	t.Log("Removed test files.")
}

func createNetwork(t *testing.T, cli *client.Client, ctx context.Context) func(*testing.T) {
	resp, err := cli.NetworkCreate(ctx, userDefinedNetwork, types.NetworkCreate{CheckDuplicate: true, Attachable: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created network:", resp.ID)

	return func(t *testing.T) {
		err := cli.NetworkRemove(ctx, resp.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("Removed network:", resp.ID)
	}
}

func getToken(t *testing.T) string {

	t.Log("Waiting for manager.")
	attempts := 1
RETRY:
	resp, err := http.Get(managerExposedEndpoint + "/health")
	if err != nil && attempts < 10 {
		t.Log(err)
		time.Sleep(5 * time.Second)
		attempts++
		goto RETRY
	} else if err != nil {
		t.Fatal("Manager not reachable.")
	}
	resp.Body.Close()

	resp, err = http.Post(managerExposedEndpoint+"/token_sets?name=test&total=1", "none", nil)
	if err != nil {
		t.Fatalf("Error requesting token: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 but got %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	respMap := make(map[string]interface{})
	err = decoder.Decode(&respMap)
	if err != nil {
		t.Fatalf("Error decoding response: %s", err)
	}

	if _, found := respMap["tokens"]; !found {
		t.Fatalf("Tokens not found in response:\n%s", spew.Sdump(respMap))
	}

	tokens, ok := respMap["tokens"].([]interface{})
	if !ok {
		t.Fatalf("Type assertion not possible for tokens in response:\n%s", spew.Sdump(respMap))
	}

	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token but got %d", len(tokens))
	}

	token, ok := tokens[0].(string)
	if !ok {
		t.Fatalf("Type assertion not possible for token in response:\n%s", spew.Sdump(respMap))
	}

	return token
}

func checkRegistration(t *testing.T) {

	resp, err := http.Get(managerExposedEndpoint + "/targets/test-agent")
	if err != nil {
		t.Fatalf("Error getting the target: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Error("Target response was not 200.")
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Error("Response:", string(b))

		resp, err := http.Get(managerExposedEndpoint + "/targets")
		if err != nil {
			t.Fatalf("Error getting list of targets: %s", err)
		}
		defer resp.Body.Close()
		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("List of targets:", string(b))
	}
}

func deployPackage(t *testing.T) {
	// download the example order from repository
	resp, err := http.Get(deployOrder)
	if err != nil {
		t.Fatal("Error downloading order:", err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		t.Fatal("Error reading downloaded order body:", err)
	}
	resp.Body.Close()

	resp, err = http.Post(managerExposedEndpoint+"/orders", "application/x-yaml", bytes.NewBuffer(b))
	if err != nil {
		t.Fatal("Error posting order:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatal("Expected status 201, but got", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	respMap := make(map[string]interface{})
	err = decoder.Decode(&respMap)
	if err != nil {
		t.Fatalf("Error decoding response: %s", err)
	}

	if _, found := respMap["id"]; !found {
		t.Fatalf("ID not found in response:\n%s", spew.Sdump(respMap))
	}

	t.Log("Created order:", respMap["id"])
}

func runElastic(t *testing.T, cli *client.Client, ctx context.Context) func(*testing.T) {

	reader, err := cli.ImagePull(ctx, elasticImage, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, err := getLastLine(reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Pulled image: %s: %s", elasticImage, status)

	// container to generate key pair
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: elasticImage,
			Env: []string{
				"discovery.type=single-node",
				"bootstrap.memory_lock=true",
				"ES_JAVA_OPTS=-Xms512m -Xmx512m",
			},
		},
		nil,
		nil,
		elasticName)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created elasticsearch container:", resp.ID)

	err = cli.NetworkConnect(ctx, userDefinedNetwork, resp.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Connected to network:", userDefinedNetwork)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Log("Started container:", resp.ID)

	return func(t *testing.T) {
		containerRemove(t, cli, ctx, resp.ID)
	}
}

func runManager(t *testing.T, cli *client.Client, ctx context.Context) func(*testing.T) {

	reader, err := cli.ImagePull(ctx, managerImage, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, err := getLastLine(reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Pulled image: %s: %s", managerImage, status)

	mountPoint := testDir + "/manager"
	err = os.MkdirAll(mountPoint, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	keysVolume := mount.Mount{
		Type:   mount.TypeBind,
		Source: mountPoint,
		Target: "/home/keys",
	}

	// container to generate key pair
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:           managerImage,
			NetworkDisabled: true,
			Cmd:             []string{"-newkeypair", "keys/manager"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{keysVolume},
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created manager container for key pair generation:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Log("Started container:", resp.ID)

	t.Log("Waiting for container to exit...")
	waitOK, _ := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	if body := <-waitOK; body.Error != nil {
		t.Fatal(body.Error)
	}
	t.Log("Container exited:", resp.ID)
	//containerLogs(t, cli, ctx, resp.ID)

	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{RemoveVolumes: true}); err != nil {
		t.Fatal(err)
	}
	t.Log("Removed container:", resp.ID)

	// actual runtime container
	resp, err = cli.ContainerCreate(ctx,
		&container.Config{
			Image: managerImage,
			Env:   []string{"STORAGE_DSN=" + elasticEndpoint, "VERBOSE=1"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{keysVolume},
			PortBindings: nat.PortMap{
				"8080/tcp": []nat.PortBinding{
					{
						HostIP:   "127.0.0.1",
						HostPort: managerPort,
					},
				},
			},
		},
		nil,
		managerName)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created manager container:", resp.ID)

	err = cli.NetworkConnect(ctx, userDefinedNetwork, resp.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Connected to network:", userDefinedNetwork)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Log("Started container:", resp.ID)

	return func(t *testing.T) {
		containerRemove(t, cli, ctx, resp.ID)
	}
}

func runAgent(t *testing.T, cli *client.Client, ctx context.Context, token string) func(*testing.T) {
	reader, err := cli.ImagePull(ctx, agentImage, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, err := getLastLine(reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Pulled image: %s: %s", agentImage, status)

	mountPoint := testDir + "/agent"
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
			Image:           agentImage,
			NetworkDisabled: true,
			Cmd:             []string{"-newkeypair", "agent"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{mountAgent},
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created agent container for key pair generation:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Log("Started container:", resp.ID)

	t.Log("Waiting for container to exit...")
	waitOK, _ := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	if body := <-waitOK; body.Error != nil {
		t.Fatal(body.Error)
	}
	t.Log("Container exited:", resp.ID)

	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{RemoveVolumes: true}); err != nil {
		t.Fatal(err)
	}
	t.Log("Removed container:", resp.ID)

	// actual runtime container
	resp, err = cli.ContainerCreate(ctx,
		&container.Config{
			Image: agentImage,
			Env: []string{
				"ID=" + "test-agent",
				"TAGS=" + "swarm",
				"AUTH_TOKEN=" + token,
				"MANAGER_ADDR=" + managerEndpoint,
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{mountAgent},
		},
		nil,
		agentName)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created agent container:", resp.ID)

	err = cli.NetworkConnect(ctx, userDefinedNetwork, resp.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Connected to network:", userDefinedNetwork)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Log("Started container:", resp.ID)

	return func(t *testing.T) {
		containerRemove(t, cli, ctx, resp.ID)
	}
}

func removeVolumes(t *testing.T, cli *client.Client, ctx context.Context) {
	imageName := "alpine"
	reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, err := getLastLine(reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Pulled image: %s: %s", imageName, status)

	volume := mount.Mount{
		Type:   mount.TypeBind,
		Source: testDir,
		Target: "/home/testdata",
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageName,
			Cmd:   []string{"rm", "-fr", "/home/testdata"},
		},
		&container.HostConfig{
			Mounts:     []mount.Mount{volume},
			AutoRemove: true,
		},
		nil,
		"")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Created cleaner container:", resp.ID)

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Log("Started container:", resp.ID)

	t.Log("Waiting for container to exit...")
	waitOK, _ := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	if body := <-waitOK; body.Error != nil {
		t.Fatal(body.Error)
	}
	t.Log("Container exited:", resp.ID)
}

func containerRemove(t *testing.T, cli *client.Client, ctx context.Context, id string) {
	if err := cli.ContainerStop(ctx, id, nil); err != nil {
		t.Fatal(err)
	}
	t.Log("Stopped container:", id)

	if t.Failed() {
		containerLogs(t, cli, ctx, id)
	}

	if err := cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		t.Fatal(err)
	}
	t.Log("Removed container:", id)
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

func getLastLine(reader io.Reader) (string, error) {
	logs, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("error reading: %s", err)
	}
	split := bytes.Split(logs, []byte("\n"))
	return string(split[len(split)-2]), nil
}
