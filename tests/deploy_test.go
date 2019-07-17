package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

const endpoint = "http://localhost:8080"

func TestDeploy(t *testing.T) {

	var err error
	var token string

	t.Run("Get a token", func(t *testing.T) {
		token, err = getToken()
		if err != nil {
			t.Fatalf("%s", err)
		}
	})
	t.Log(token)

	t.Run("Run an agent", func(t *testing.T) {
		err = runAgent(token)
		if err != nil {
			t.Fatalf("%s", err)
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

func runAgent(token string) error {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	imageName := "linksmart/deployment-agent"

	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, out)

	workDir, _ := os.Getwd()
	mountPoint := workDir + "/agent-vol"
	err = os.MkdirAll(mountPoint, os.ModePerm)
	if err != nil {
		panic(err)
	}
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageName,
			//Env:   []string{"AUTH_TOKEN=" + token, "MANAGER_ADDR=" + endpoint},
			Cmd: []string{"-newkeypair", "agent"},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: mountPoint,
					Target: "/home/agent",
				},
			},
		}, nil, "test-agent")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	time.Sleep(3 * time.Second)
	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
		panic(err)
	}

	fmt.Println(resp.ID)
	return nil
}
