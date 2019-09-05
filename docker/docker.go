package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jhoonb/archivex"
	"github.com/pivotal/deplab/metadata"
)

func CreateNewImage(inputImage string, md metadata.Metadata, tag string) (resp types.ImageBuildResponse, err error) {
	dockerCli, err := client.NewClientWithOpts(client.WithVersion("1.39"), client.FromEnv)
	if err != nil {
		return resp, err
	}

	dockerfileBuffer, err := createDockerFileBuffer(inputImage)
	if err != nil {
		return resp, err
	}

	mdMarshalled, err := json.Marshal(md)
	if err != nil {
		return resp, err
	}

	opt := types.ImageBuildOptions{
		Labels: map[string]string{
			"io.pivotal.metadata": string(mdMarshalled),
		},
	}

	if tag != "" {
		_, err := reference.ParseAnyReference(tag)
		if err != nil {
			return resp, fmt.Errorf("tag %s is not valid: %s", tag, err)
		}
		opt.Tags = []string{tag}
	}

	resp, err = dockerCli.ImageBuild(context.Background(), &dockerfileBuffer, opt)
	return resp, err
}

func createDockerFileBuffer(inputImage string) (bytes.Buffer, error) {
	dockerfileBuffer := bytes.Buffer{}

	t := new(archivex.TarFile)
	err := t.CreateWriter("docker context", &dockerfileBuffer)
	if err != nil {
		return dockerfileBuffer, fmt.Errorf("error creating tar writer: %s\n", err.Error())
	}
	err = t.Add("Dockerfile", strings.NewReader("FROM "+inputImage), nil)
	if err != nil {
		return dockerfileBuffer, fmt.Errorf("error adding to the tar: %s\n", err.Error())
	}
	err = t.Close()
	if err != nil {
		return dockerfileBuffer, fmt.Errorf("error closing the tar: %s\n", err.Error())
	}

	return dockerfileBuffer, nil
}

func GetIDOfNewImage(resp types.ImageBuildResponse) (string, error) {
	rd := json.NewDecoder(resp.Body)

	for {
		line := struct {
			Aux struct {
				ID string
			}
			Stream string
			Error  string
		}{}

		err := rd.Decode(&line)
		if err == io.EOF {
			return "", fmt.Errorf("could not find the new image ID")
		} else if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error reading line")
			continue
		}

		if line.Error != "" {
			return "", fmt.Errorf("error building image: %s\n", line.Error)
		} else if line.Aux.ID != "" {
			return line.Aux.ID, nil
		}
	}
}

func ReadFromImage(imageName, path string) (*tar.Reader, error) {
	createContainerCmd := exec.Command("docker", "create", imageName, "foo")
	//defer image cleanup

	containerIdBytes, err := createContainerCmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to create container: %s with error: %s", e.Stderr, err)
		}
		return nil, fmt.Errorf("failed to create container: %s", err)
	}

	containerId := strings.TrimSpace(string(containerIdBytes))

	c := exec.Command("docker", "cp", "-L", fmt.Sprintf("%s:%s", containerId, path), "-")
	var cOut bytes.Buffer
	c.Stdout = &cOut

	err = c.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting docker cp command to retrieve %s: %s", path, err)
	}

	err = c.Wait()
	if err != nil {
		var buf bytes.Buffer
		return tar.NewReader(&buf), nil
	}

	return tar.NewReader(&cOut), nil
}